package display

import (
	"bytes"
	"log/slog"
	"testing"

	log "github.com/k0sproject/k0sctl/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlainDisplayRendersVisibleRecords(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewPlain(&buf, slog.LevelInfo, false))

	logger.Info("visible", log.KeyHost, "h1")

	assert.Equal(t, "INFO h1: visible\n", buf.String())
}

func TestPlainDisplayCapturesBelowLevelRecordsInRings(t *testing.T) {
	var buf bytes.Buffer
	d := NewPlain(&buf, slog.LevelInfo, false)
	logger := slog.New(d)

	logger.Debug("hidden detail", log.KeyHost, "h1")

	assert.Empty(t, buf.String(), "below-level record must not render")
	tail := d.st.rings.tail("h1", 5)
	require.Len(t, tail, 1)
	assert.Equal(t, "hidden detail", tail[0].Message)
}

func TestPlainDisplayWithAttrsRoutesToRings(t *testing.T) {
	var buf bytes.Buffer
	d := NewPlain(&buf, slog.LevelInfo, false)
	logger := slog.New(d).With(log.KeyHost, "scoped")

	logger.Info("via scoped logger")

	assert.Equal(t, "INFO scoped: via scoped logger\n", buf.String())
	require.Len(t, d.st.rings.tail("scoped", 5), 1)
}

func TestPlainDisplayDumpsFailedHostTailOnFatal(t *testing.T) {
	var buf bytes.Buffer
	d := NewPlain(&buf, slog.LevelInfo, false)
	logger := slog.New(d)

	logger.Debug("step one", log.KeyHost, "h1")
	logger.Error("phase failed", log.KeyHost, "h1", log.KeyError, "kaboom")
	logger.Info("unrelated", log.KeyHost, "h2")
	buf.Reset()

	logger.Log(t.Context(), log.LevelFatal, "run failed")

	out := buf.String()
	assert.Contains(t, out, "last 2 log entries for host h1:")
	assert.Contains(t, out, "step one")
	assert.Contains(t, out, "phase failed error=kaboom")
	assert.NotContains(t, out, "h2", "healthy hosts must not be dumped")
	assert.Contains(t, out, "FATA run failed")
	// the dump must precede the fatal message
	assert.Less(t, bytes.Index(buf.Bytes(), []byte("last 2 log entries")), bytes.Index(buf.Bytes(), []byte("FATA")))

	// a second fatal must not dump again
	buf.Reset()
	logger.Log(t.Context(), log.LevelFatal, "again")
	assert.NotContains(t, buf.String(), "last 2 log entries")
}

func TestPlainDisplayNoDumpWithoutFailures(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewPlain(&buf, slog.LevelInfo, false))

	logger.Info("all good", log.KeyHost, "h1")
	logger.Log(t.Context(), log.LevelFatal, "fatal anyway")

	assert.NotContains(t, buf.String(), "log entries for host")
}
