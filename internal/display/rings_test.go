package display

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hostEvent(host string, level slog.Level, msg string) Event {
	return Event{Host: host, Level: level, Message: msg}
}

func TestRingWrapsAroundKeepingMostRecent(t *testing.T) {
	rs := newRings()
	for i := range ringSize + 10 {
		rs.add(hostEvent("h1", slog.LevelDebug, fmt.Sprintf("msg-%d", i)))
	}

	tail := rs.tail("h1", 3)
	require.Len(t, tail, 3)
	assert.Equal(t, fmt.Sprintf("msg-%d", ringSize+7), tail[0].Message)
	assert.Equal(t, fmt.Sprintf("msg-%d", ringSize+8), tail[1].Message)
	assert.Equal(t, fmt.Sprintf("msg-%d", ringSize+9), tail[2].Message)
}

func TestRingTailShorterThanRequested(t *testing.T) {
	rs := newRings()
	rs.add(hostEvent("h1", slog.LevelInfo, "only"))

	tail := rs.tail("h1", 10)
	require.Len(t, tail, 1)
	assert.Equal(t, "only", tail[0].Message)
}

func TestRingsIgnoreHostlessEvents(t *testing.T) {
	rs := newRings()
	rs.add(Event{Level: slog.LevelError, Message: "no host"})

	assert.Empty(t, rs.hosts)
	assert.Nil(t, rs.failures(5))
}

func TestRingsTrackFailedHostsOnce(t *testing.T) {
	rs := newRings()
	rs.add(hostEvent("h1", slog.LevelError, "fail 1"))
	rs.add(hostEvent("h1", slog.LevelError, "fail 2"))
	rs.add(hostEvent("h2", slog.LevelInfo, "fine"))

	assert.Equal(t, []string{"h1"}, rs.failed)

	failures := rs.failures(5)
	require.Len(t, failures, 1)
	require.Len(t, failures["h1"], 2)
	assert.Equal(t, "fail 1", failures["h1"][0].Message)
}

func TestRingsTailUnknownHost(t *testing.T) {
	rs := newRings()
	assert.Nil(t, rs.tail("nope", 3))
}
