package log

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScreenHandlerBasicFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, false))

	logger.Info("hello world")

	assert.Equal(t, "INFO hello world\n", buf.String())
}

func TestScreenHandlerLevelTags(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
		tag   string
	}{
		{"trace", LevelTrace, "TRAC"},
		{"debug", slog.LevelDebug, "DEBU"},
		{"info", slog.LevelInfo, "INFO"},
		{"warn", slog.LevelWarn, "WARN"},
		{"error", slog.LevelError, "ERRO"},
		{"fatal", LevelFatal, "FATA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(NewScreenHandler(&buf, LevelTrace, false))

			logger.Log(context.Background(), tt.level, "msg")

			want := tt.tag + " msg\n"
			assert.Equal(t, want, buf.String())
		})
	}
}

func TestScreenHandlerHostAttrRendersAsPrefix(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, false))

	logger.Info("connecting", KeyHost, "10.0.0.1")

	assert.Equal(t, "INFO 10.0.0.1: connecting\n", buf.String())
}

func TestScreenHandlerHostAttrFromWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, false)).With(KeyHost, "node1")

	logger.Info("connected")

	assert.Equal(t, "INFO node1: connected\n", buf.String())
}

func TestScreenHandlerOtherAttrsRenderTrailing(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, false))

	logger.Info("did something", "phase", "apply")

	assert.Equal(t, `INFO did something phase="apply"`+"\n", buf.String())
}

func TestScreenHandlerHostAndTrailingAttrsCombined(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, false))

	logger.Info("msg", KeyHost, "node1", "phase", "apply")

	assert.Equal(t, `INFO node1: msg phase="apply"`+"\n", buf.String())
}

func TestScreenHandlerDropsEmptyErrorAttr(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, false))

	logger.Info("ok", KeyError, "")

	assert.Equal(t, "INFO ok\n", buf.String())
}

func TestScreenHandlerKeepsNonEmptyErrorAttr(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, false))

	logger.Error("failed", KeyError, "boom")

	assert.Equal(t, `ERRO failed error="boom"`+"\n", buf.String())
}

func TestScreenHandlerColorsOff(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, false))

	logger.Warn("careful")

	assert.NotContains(t, buf.String(), "\x1b[")
}

func TestScreenHandlerColorsOn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, true))

	logger.Warn("careful")

	out := buf.String()
	assert.Contains(t, out, ansiYellow, "warn level should be colored yellow")
	assert.Contains(t, out, ansiReset)
}

func TestScreenHandlerEnabledRespectsLevel(t *testing.T) {
	h := NewScreenHandler(io.Discard, slog.LevelWarn, false)

	assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, h.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, h.Enabled(context.Background(), slog.LevelError))
}

func TestScreenHandlerFiltersRecordsBelowLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelWarn, false))

	logger.Info("should not appear")
	logger.Warn("should appear")

	assert.Equal(t, "WARN should appear\n", buf.String())
}

func TestScreenHandlerWithGroupFlattensInsteadOfNesting(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewScreenHandler(&buf, slog.LevelDebug, false)).WithGroup("g").With("k", "v")

	logger.Info("msg")

	assert.Equal(t, `INFO msg k="v"`+"\n", buf.String())
}

func TestFileHandlerLogfmtOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewFileHandler(&buf, slog.LevelDebug))

	logger.Info("hello", KeyHost, "node1")

	out := buf.String()
	assert.Contains(t, out, "level=INFO")
	assert.Contains(t, out, "msg=hello")
	assert.Contains(t, out, "host=node1")
}

func TestFileHandlerCustomLevelNames(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewFileHandler(&buf, LevelTrace))

	logger.Log(context.Background(), LevelTrace, "trace msg")
	logger.Log(context.Background(), LevelFatal, "fatal msg")

	out := buf.String()
	assert.Contains(t, out, "level=TRACE")
	assert.Contains(t, out, "level=FATAL")
	assert.NotContains(t, out, "DEBUG-4")
	assert.NotContains(t, out, "ERROR+4")
}

func TestFileHandlerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewFileHandler(&buf, slog.LevelWarn))

	logger.Info("skip me")
	logger.Warn("keep me")

	out := buf.String()
	assert.NotContains(t, out, "skip me")
	assert.Contains(t, out, "keep me")
}

func TestFanoutHandlerForwardsToAllEnabledHandlers(t *testing.T) {
	var screenBuf, fileBuf bytes.Buffer
	h1 := NewScreenHandler(&screenBuf, slog.LevelDebug, false)
	h2 := NewFileHandler(&fileBuf, slog.LevelDebug)
	logger := slog.New(NewFanoutHandler(h1, h2))

	logger.Info("hello")

	assert.Contains(t, screenBuf.String(), "hello")
	assert.Contains(t, fileBuf.String(), "msg=hello")
}

func TestFanoutHandlerRespectsIndividualHandlerLevels(t *testing.T) {
	var screenBuf, fileBuf bytes.Buffer
	h1 := NewScreenHandler(&screenBuf, slog.LevelInfo, false) // filters debug
	h2 := NewFileHandler(&fileBuf, slog.LevelDebug)           // allows debug
	logger := slog.New(NewFanoutHandler(h1, h2))

	logger.Debug("debug message")

	assert.Empty(t, screenBuf.String(), "handler above the record's level should not receive it")
	assert.Contains(t, fileBuf.String(), "debug message")
}

func TestFanoutHandlerEnabledIfAnyHandlerEnabled(t *testing.T) {
	strict := NewScreenHandler(io.Discard, slog.LevelError, false)
	lenient := NewFileHandler(io.Discard, slog.LevelDebug)

	both := NewFanoutHandler(strict, lenient)
	assert.True(t, both.Enabled(context.Background(), slog.LevelDebug), "fanout should be enabled if any handler is enabled")

	onlyStrict := NewFanoutHandler(strict)
	assert.False(t, onlyStrict.Enabled(context.Background(), slog.LevelDebug))
}

func TestFanoutHandlerWithAttrsPropagatesToAllHandlers(t *testing.T) {
	var screenBuf, fileBuf bytes.Buffer
	h1 := NewScreenHandler(&screenBuf, slog.LevelDebug, false)
	h2 := NewFileHandler(&fileBuf, slog.LevelDebug)
	logger := slog.New(NewFanoutHandler(h1, h2)).With(KeyHost, "node1")

	logger.Info("connected")

	assert.Contains(t, screenBuf.String(), "node1: connected")
	assert.Contains(t, fileBuf.String(), "host=node1")
}

func TestFanoutHandlerStillForwardsToOtherHandlersWhenOneFails(t *testing.T) {
	failing := &alwaysErrorHandler{}
	var fileBuf bytes.Buffer
	ok := NewFileHandler(&fileBuf, slog.LevelDebug)

	fanout := NewFanoutHandler(failing, ok)
	err := fanout.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0))

	require.Error(t, err, "fanout should surface the failing handler's error")
	assert.Contains(t, fileBuf.String(), "msg=msg", "the other handler should still receive the record")
}

// alwaysErrorHandler is a slog.Handler that always fails, used to verify the
// fanout handler still forwards to the remaining handlers and joins errors
// rather than aborting.
type alwaysErrorHandler struct{}

func (a *alwaysErrorHandler) Enabled(context.Context, slog.Level) bool { return true }
func (a *alwaysErrorHandler) Handle(context.Context, slog.Record) error {
	return errors.New("always fails")
}
func (a *alwaysErrorHandler) WithAttrs(_ []slog.Attr) slog.Handler { return a }
func (a *alwaysErrorHandler) WithGroup(_ string) slog.Handler     { return a }
