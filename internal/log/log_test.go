package log

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRecord is a flattened view of a captured slog.Record, merging both the
// handler's accumulated attrs (from WithAttrs) and the record's own attrs, in
// the same way a real handler would when rendering output.
type testRecord struct {
	level   slog.Level
	message string
	attrs   []slog.Attr
}

func (r testRecord) attr(key string) (slog.Attr, bool) {
	for _, a := range r.attrs {
		if a.Key == key {
			return a, true
		}
	}
	return slog.Attr{}, false
}

// recordingHandler is a minimal slog.Handler that records every record it
// receives, along with any attrs attached via WithAttrs, so tests can assert
// on what package-level funcs and Logger methods actually emit.
type recordingHandler struct {
	mu      *sync.Mutex
	records *[]testRecord
	attrs   []slog.Attr
}

func newRecordingHandler() *recordingHandler {
	return &recordingHandler{mu: &sync.Mutex{}, records: &[]testRecord{}}
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := append([]slog.Attr{}, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, testRecord{level: r.Level, message: r.Message, attrs: attrs})
	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *recordingHandler) WithGroup(_ string) slog.Handler { return h }

func (h *recordingHandler) Records() []testRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]testRecord, len(*h.records))
	copy(out, *h.records)
	return out
}

// installRecordingHandler swaps the package base logger for one backed by a
// recordingHandler and restores the original when the test ends.
func installRecordingHandler(t *testing.T) *recordingHandler {
	t.Helper()
	orig := Base()
	t.Cleanup(func() { SetLogger(orig) })

	h := newRecordingHandler()
	SetLogger(slog.New(h))
	return h
}

func TestLevelConstants(t *testing.T) {
	assert.Equal(t, slog.LevelDebug-4, LevelTrace, "LevelTrace should be 4 below LevelDebug")
	assert.Equal(t, slog.LevelError+4, LevelFatal, "LevelFatal should be 4 above LevelError")
}

func TestPackageLevelFormattedFuncsRouteAtCorrectLevels(t *testing.T) {
	h := installRecordingHandler(t)

	Tracef("trace %d", 1)
	Debugf("debug %d", 2)
	Infof("info %d", 3)
	Warnf("warn %d", 4)
	Errorf("error %d", 5)

	recs := h.Records()
	require.Len(t, recs, 5)

	want := []struct {
		level slog.Level
		msg   string
	}{
		{LevelTrace, "trace 1"},
		{slog.LevelDebug, "debug 2"},
		{slog.LevelInfo, "info 3"},
		{slog.LevelWarn, "warn 4"},
		{slog.LevelError, "error 5"},
	}
	for i, w := range want {
		assert.Equal(t, w.level, recs[i].level, "record %d level", i)
		assert.Equal(t, w.msg, recs[i].message, "record %d message", i)
	}
}

func TestPackageLevelFuncsRouteAtCorrectLevels(t *testing.T) {
	h := installRecordingHandler(t)

	Trace("trace", "-1")
	Debug("debug", "-2")
	Info("info", "-3")
	Warn("warn", "-4")
	Error("error", "-5")

	recs := h.Records()
	require.Len(t, recs, 5)

	want := []struct {
		level slog.Level
		msg   string
	}{
		{LevelTrace, "trace-1"},
		{slog.LevelDebug, "debug-2"},
		{slog.LevelInfo, "info-3"},
		{slog.LevelWarn, "warn-4"},
		{slog.LevelError, "error-5"},
	}
	for i, w := range want {
		assert.Equal(t, w.level, recs[i].level, "record %d level", i)
		assert.Equal(t, w.msg, recs[i].message, "record %d message", i)
	}
}

func TestSetLoggerAndBase(t *testing.T) {
	orig := Base()
	t.Cleanup(func() { SetLogger(orig) })

	l := slog.New(newRecordingHandler())
	SetLogger(l)
	assert.Same(t, l, Base())
}

func TestWithAttachesAttrs(t *testing.T) {
	h := installRecordingHandler(t)

	With(KeyHost, "node1").Info("hello")

	recs := h.Records()
	require.Len(t, recs, 1)
	assert.Equal(t, "hello", recs[0].message)

	a, ok := recs[0].attr(KeyHost)
	require.True(t, ok, "expected host attr to be present")
	assert.Equal(t, "node1", a.Value.String())
}

func TestLoggerWithChains(t *testing.T) {
	h := installRecordingHandler(t)

	With(KeyHost, "node1").With("phase", "apply").Warnf("uh oh %d", 1)

	recs := h.Records()
	require.Len(t, recs, 1)
	assert.Equal(t, slog.LevelWarn, recs[0].level)
	assert.Equal(t, "uh oh 1", recs[0].message)

	host, ok := recs[0].attr(KeyHost)
	require.True(t, ok, "expected host attr from the first With call")
	assert.Equal(t, "node1", host.Value.String())

	phase, ok := recs[0].attr("phase")
	require.True(t, ok, "expected phase attr from the chained With call")
	assert.Equal(t, "apply", phase.Value.String())
}

func TestLoggerWithDoesNotMutateParent(t *testing.T) {
	h := installRecordingHandler(t)

	base := With(KeyHost, "node1")
	child := base.With("phase", "apply")

	base.Info("from base")
	child.Info("from child")

	recs := h.Records()
	require.Len(t, recs, 2)

	if _, ok := recs[0].attr("phase"); ok {
		t.Errorf("base logger record should not carry the phase attr added only to the child")
	}
	if _, ok := recs[1].attr("phase"); !ok {
		t.Errorf("child logger record should carry the phase attr")
	}
}

func TestLoggerSlogReturnsUnderlying(t *testing.T) {
	h := installRecordingHandler(t)

	l := With(KeyHost, "node1")
	l.Slog().Info("via slog")

	recs := h.Records()
	require.Len(t, recs, 1)
	assert.Equal(t, "via slog", recs[0].message)
	_, ok := recs[0].attr(KeyHost)
	assert.True(t, ok, "attrs attached via With should still apply when using Slog() directly")
}

func TestIntoContextFromContextRoundTrip(t *testing.T) {
	h := installRecordingHandler(t)

	l := With(KeyHost, "node1")
	ctx := IntoContext(context.Background(), l)

	got := FromContext(ctx)
	require.Same(t, l, got, "FromContext should return the exact logger stored by IntoContext")

	got.Info("via context")

	recs := h.Records()
	require.Len(t, recs, 1)
	a, ok := recs[0].attr(KeyHost)
	require.True(t, ok)
	assert.Equal(t, "node1", a.Value.String())
}

func TestFromContextFallsBackToBaseLogger(t *testing.T) {
	h := installRecordingHandler(t)

	got := FromContext(context.Background())
	got.Info("fallback")

	recs := h.Records()
	require.Len(t, recs, 1)
	assert.Equal(t, "fallback", recs[0].message)
}

func TestFromContextIgnoresValuesOfTheWrongType(t *testing.T) {
	h := installRecordingHandler(t)

	// A value stored under the same key type but wrong dynamic type should
	// not be mistaken for a *Logger.
	ctx := context.WithValue(context.Background(), ctxKey{}, "not-a-logger")
	got := FromContext(ctx)
	got.Info("fallback2")

	recs := h.Records()
	require.Len(t, recs, 1)
	assert.Equal(t, "fallback2", recs[0].message)
}

func TestLevelName(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
		want  string
	}{
		{"trace", LevelTrace, "TRACE"},
		{"below trace", LevelTrace - 100, "TRACE"},
		{"debug", slog.LevelDebug, "DEBUG"},
		{"info", slog.LevelInfo, "INFO"},
		{"warn", slog.LevelWarn, "WARN"},
		{"error", slog.LevelError, "ERROR"},
		{"fatal", LevelFatal, "FATAL"},
		{"above fatal", LevelFatal + 100, "FATAL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, levelName(tt.level), "levelName(%v)", tt.level)
		})
	}
}
