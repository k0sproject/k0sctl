package display

import (
	"log/slog"
	"testing"
	"time"

	log "github.com/k0sproject/k0sctl/internal/log"
	"github.com/stretchr/testify/assert"
)

func record(t *testing.T, level slog.Level, msg string, args ...any) slog.Record {
	t.Helper()
	r := slog.NewRecord(time.Date(2026, 7, 2, 12, 34, 56, 0, time.Local), level, msg, 0)
	r.Add(args...)
	return r
}

func TestParseEventRoutingAttrs(t *testing.T) {
	r := record(t, slog.LevelInfo, "hello",
		log.KeyHost, "node1:22",
		log.KeyPhase, "Connect",
		log.KeyDuration, 1500*time.Millisecond,
		log.KeyAttempt, int64(3),
		log.KeyError, "boom",
	)

	ev := parseEvent(nil, r)

	assert.Equal(t, "hello", ev.Message)
	assert.Equal(t, "node1:22", ev.Host)
	assert.Equal(t, "Connect", ev.Phase)
	assert.Equal(t, 1500*time.Millisecond, ev.Duration)
	assert.Equal(t, int64(3), ev.Attempt)
	assert.Equal(t, "boom", ev.Err)
	assert.Empty(t, ev.Rest)
}

func TestParseEventHandlerScopedAttrsWinOverRecordAttrs(t *testing.T) {
	scoped := []slog.Attr{slog.String(log.KeyHost, "scoped-host")}
	r := record(t, slog.LevelInfo, "msg", log.KeyHost, "record-host")

	ev := parseEvent(scoped, r)

	assert.Equal(t, "scoped-host", ev.Host)
}

func TestParseEventUnknownAttrsLandInRest(t *testing.T) {
	r := record(t, slog.LevelDebug, "exec", "command", "uptime", "sudo", "true")

	ev := parseEvent(nil, r)

	assert.Equal(t, []string{"command=uptime", "sudo=true"}, ev.Rest)
}

func TestEventLine(t *testing.T) {
	r := record(t, slog.LevelInfo, "retrying",
		log.KeyAttempt, int64(4),
		log.KeyError, "connection refused",
		"component", "test",
	)

	line := parseEvent(nil, r).line()

	assert.Equal(t, "12:34:56 retrying attempt=4 error=connection refused component=test", line)
}
