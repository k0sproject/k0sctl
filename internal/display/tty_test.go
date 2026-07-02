package display

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testModel builds a ttyModel without starting a bubbletea program.
func testModel(t *testing.T, interactive bool, onInterrupt func()) *ttyModel {
	t.Helper()
	st := &state{rings: newRings(), out: &bytes.Buffer{}, level: slog.LevelInfo}
	r := newTTYRenderer(st, &bytes.Buffer{}, interactive, onInterrupt)
	return newTTYModel(r)
}

func update(t *testing.T, m *ttyModel, msg tea.Msg) tea.Cmd {
	t.Helper()
	_, cmd := m.Update(msg)
	return cmd
}

func phaseStart(title string) Event {
	return Event{Phase: title, Level: slog.LevelInfo, Time: time.Now(), Message: "==> Running phase: " + title}
}

func phaseEnd(title string, err string) Event {
	return Event{Phase: title, Level: slog.LevelDebug, Duration: 2 * time.Second, Err: err, Message: "phase completed"}
}

func TestTTYModelPhaseStartShowsLiveRegion(t *testing.T) {
	m := testModel(t, false, nil)

	cmd := update(t, m, eventMsg{ev: phaseStart("Connect to hosts")})

	assert.Nil(t, cmd)
	assert.Contains(t, m.View(), "Connect to hosts")
}

func TestTTYModelPhaseCompletionClearsLiveRegion(t *testing.T) {
	m := testModel(t, false, nil)
	update(t, m, eventMsg{ev: phaseStart("Connect to hosts")})

	cmd := update(t, m, eventMsg{ev: phaseEnd("Connect to hosts", "")})

	require.NotNil(t, cmd, "phase completion must persist a line via tea.Println")
	assert.Empty(t, m.View())
}

func TestTTYModelHostRowsInOrderWithRetryCounter(t *testing.T) {
	m := testModel(t, false, nil)
	update(t, m, eventMsg{ev: phaseStart("Upgrade")})
	update(t, m, eventMsg{ev: Event{Host: "h1", Level: slog.LevelInfo, Message: "starting upgrade"}})
	update(t, m, eventMsg{ev: Event{Host: "h2", Level: slog.LevelInfo, Message: "waiting"}})
	// retry event keeps the last status message, adds the counter
	update(t, m, eventMsg{ev: Event{Host: "h1", Level: slog.LevelDebug, Message: "retrying", Attempt: 4, Err: "conn refused"}})

	assert.Equal(t, []string{"h1", "h2"}, m.order)
	view := m.View()
	assert.Contains(t, view, "starting upgrade")
	assert.Contains(t, view, "⟳ 4")
	assert.Contains(t, view, "conn refused")

	// a regular event clears the retry counter again
	update(t, m, eventMsg{ev: Event{Host: "h1", Level: slog.LevelInfo, Message: "service started"}})
	assert.NotContains(t, m.View(), "⟳")
}

func TestTTYModelHostErrorMarksRow(t *testing.T) {
	m := testModel(t, false, nil)
	update(t, m, eventMsg{ev: phaseStart("Upgrade")})

	cmd := update(t, m, eventMsg{ev: Event{Host: "h1", Level: slog.LevelError, Message: "phase failed", Err: "kaboom"}})

	assert.Nil(t, cmd, "host records must not persist lines")
	assert.Equal(t, slog.LevelError, m.rows["h1"].level)
	assert.Contains(t, m.View(), "h1")
}

func TestTTYModelNonHostRecordsPersist(t *testing.T) {
	m := testModel(t, false, nil)

	assert.NotNil(t, update(t, m, eventMsg{ev: Event{Level: slog.LevelInfo, Message: "info line"}}))
	assert.NotNil(t, update(t, m, eventMsg{ev: Event{Level: slog.LevelWarn, Message: "warn line"}}))
	assert.NotNil(t, update(t, m, eventMsg{ev: Event{Level: slog.LevelError, Message: "error line"}}))
	assert.Nil(t, update(t, m, eventMsg{ev: Event{Level: slog.LevelDebug, Message: "debug line"}}))
}

func TestTTYModelTailKeys(t *testing.T) {
	m := testModel(t, true, nil)
	update(t, m, eventMsg{ev: phaseStart("Upgrade")})
	for i := range 6 {
		update(t, m, eventMsg{ev: Event{Host: fmt.Sprintf("h%d", i), Level: slog.LevelInfo, Message: "working"}})
	}

	update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	assert.True(t, m.tailsAll)

	update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	assert.Equal(t, 1, m.focus)
	assert.False(t, m.tailsAll)

	// same key again toggles focus off
	update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	assert.Equal(t, -1, m.focus)

	// out of range is ignored
	update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("9")})
	assert.Equal(t, -1, m.focus)
}

func TestTTYModelCtrlCInterruptsThenForces(t *testing.T) {
	interrupted := false
	m := testModel(t, true, func() { interrupted = true })

	cmd := update(t, m, tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd, "first ctrl-c must print the abort notice")
	assert.True(t, interrupted)
	assert.False(t, m.t.forceExit)

	cmd = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd)
	assert.True(t, m.t.forceExit)
}

func TestTTYModelManyHostsCapped(t *testing.T) {
	m := testModel(t, false, nil)
	update(t, m, eventMsg{ev: phaseStart("Upgrade")})
	for i := range maxRows + 3 {
		update(t, m, eventMsg{ev: Event{Host: fmt.Sprintf("host-%02d", i), Level: slog.LevelInfo, Message: "working"}})
	}

	assert.Contains(t, m.View(), "… 3 more hosts")
}

func TestTTYModelStopQuits(t *testing.T) {
	m := testModel(t, false, nil)
	update(t, m, eventMsg{ev: phaseStart("Upgrade")})

	cmd := update(t, m, stopMsg{})

	require.NotNil(t, cmd)
	assert.Empty(t, m.View())
}
