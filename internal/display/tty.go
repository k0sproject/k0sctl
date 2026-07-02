package display

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	// autoTailHosts: per-host log tails show automatically when at most
	// this many hosts are active in the current phase
	autoTailHosts = 4
	// tailLines is how many recent records a host tail shows
	tailLines = 3
	// peekTailLines is the deeper tail shown in peek mode (space key)
	peekTailLines = 10
	// maxRows caps the host rows rendered in the live region
	maxRows = 12
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type (
	eventMsg struct{ ev Event }
	stopMsg  struct{}
	tickMsg  struct{}
)

// ttyRenderer runs the live bubbletea view. It starts lazily on the first
// event and stops before the process exits or a fatal record is printed.
type ttyRenderer struct {
	st          *state
	out         io.Writer
	interactive bool
	onInterrupt func()

	mu        sync.Mutex
	prog      *tea.Program
	started   bool
	stopped   bool
	forceExit bool
	done      chan struct{}
}

func newTTYRenderer(st *state, out io.Writer, interactive bool, onInterrupt func()) *ttyRenderer {
	return &ttyRenderer{st: st, out: out, interactive: interactive, onInterrupt: onInterrupt, done: make(chan struct{})}
}

func (t *ttyRenderer) running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.stopped
}

func (t *ttyRenderer) hasStarted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.started
}

func (t *ttyRenderer) send(ev Event) {
	t.mu.Lock()
	if t.stopped {
		t.mu.Unlock()
		return
	}
	if !t.started {
		t.start()
	}
	prog := t.prog
	t.mu.Unlock()
	prog.Send(eventMsg{ev: ev})
}

// start launches the bubbletea program. Callers must hold t.mu.
func (t *ttyRenderer) start() {
	opts := []tea.ProgramOption{tea.WithOutput(t.out), tea.WithoutSignalHandler()}
	if !t.interactive {
		opts = append(opts, tea.WithInput(nil))
	}
	t.prog = tea.NewProgram(newTTYModel(t), opts...)
	t.started = true
	go func() {
		_, err := t.prog.Run()
		t.mu.Lock()
		t.stopped = true
		force := t.forceExit
		t.mu.Unlock()
		close(t.done)
		if force {
			os.Exit(130)
		}
		if err != nil {
			fmt.Fprintf(t.out, "display error: %v\n", err)
		}
	}()
}

// stop shuts down the live view and waits for the terminal to be restored.
func (t *ttyRenderer) stop() {
	t.mu.Lock()
	if !t.started || t.stopped {
		t.stopped = true
		t.mu.Unlock()
		return
	}
	prog := t.prog
	t.mu.Unlock()
	prog.Send(stopMsg{})
	select {
	case <-t.done:
	case <-time.After(2 * time.Second):
		prog.Kill()
		<-t.done
	}
}

type hostRow struct {
	host    string
	msg     string
	attempt int64
	err     string
	level   slog.Level
}

type ttyModel struct {
	t     *ttyRenderer
	width int
	spin  int

	phase      string
	phaseStep  int64
	phaseTotal int64
	phaseStart time.Time

	order []string
	rows  map[string]*hostRow

	tailsAll   bool
	peek       bool // deeper tails for all hosts, toggled with space
	focus      int  // index into order, -1 = none
	interrupts int

	styleDim    lipgloss.Style
	styleGreen  lipgloss.Style
	styleRed    lipgloss.Style
	styleYellow lipgloss.Style
	styleCyan   lipgloss.Style
}

func newTTYModel(t *ttyRenderer) *ttyModel {
	r := lipgloss.NewRenderer(t.out)
	return &ttyModel{
		t:           t,
		width:       80,
		rows:        map[string]*hostRow{},
		focus:       -1,
		styleDim:    r.NewStyle().Faint(true),
		styleGreen:  r.NewStyle().Foreground(lipgloss.Color("2")),
		styleRed:    r.NewStyle().Foreground(lipgloss.Color("1")),
		styleYellow: r.NewStyle().Foreground(lipgloss.Color("3")),
		styleCyan:   r.NewStyle().Foreground(lipgloss.Color("6")),
	}
}

func tick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *ttyModel) Init() tea.Cmd {
	return tick()
}

func (m *ttyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tickMsg:
		m.spin++
		return m, tick()
	case stopMsg:
		m.phase = ""
		return m, tea.Quit
	case tea.KeyMsg:
		return m.handleKey(msg)
	case eventMsg:
		return m.handleEvent(msg.ev)
	}
	return m, nil
}

func (m *ttyModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c":
		m.interrupts++
		if m.interrupts == 1 {
			if m.t.onInterrupt != nil {
				m.t.onInterrupt()
			}
			return m, tea.Println(m.styleYellow.Render("Aborting... Press Ctrl-C again to exit now."))
		}
		m.t.mu.Lock()
		m.t.forceExit = true
		m.t.mu.Unlock()
		return m, tea.Quit
	case "l":
		m.tailsAll = !m.tailsAll
		m.focus = -1
	case " ":
		m.peek = !m.peek
	case "esc", "0":
		m.focus = -1
		m.tailsAll = false
		m.peek = false
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			idx := int(key[0] - '1')
			if idx < len(m.order) {
				if m.focus == idx {
					m.focus = -1
				} else {
					m.focus = idx
					m.tailsAll = false
				}
			}
		}
	}
	return m, nil
}

func (m *ttyModel) handleEvent(ev Event) (tea.Model, tea.Cmd) {
	// phase completion record: persist a result line above the live region
	if ev.Phase != "" && ev.Duration > 0 {
		var line string
		if ev.Err != "" {
			// keep it on one line: multi-line content confuses the live
			// region repaint, and the full error follows in the final output
			line = m.styleRed.Render(fmt.Sprintf("✘ %s (%s): %s", ev.Phase, ev.Duration.Truncate(100*time.Millisecond), firstLine(ev.Err)))
		} else {
			line = m.styleGreen.Render("✔ ") + ev.Phase + m.styleDim.Render(fmt.Sprintf(" (%s)", ev.Duration.Truncate(100*time.Millisecond)))
		}
		if ev.Phase == m.phase {
			m.phase = ""
			m.order = nil
			m.rows = map[string]*hostRow{}
			m.focus = -1
		}
		return m, tea.Println(line)
	}

	// phase start record
	if ev.Phase != "" && ev.Level == slog.LevelInfo {
		m.phase = ev.Phase
		m.phaseStep = ev.Step
		m.phaseTotal = ev.Total
		m.phaseStart = ev.Time
		m.order = nil
		m.rows = map[string]*hostRow{}
		m.focus = -1
		return m, nil
	}

	if ev.Host != "" {
		row, ok := m.rows[ev.Host]
		if !ok {
			row = &hostRow{host: ev.Host}
			m.rows[ev.Host] = row
			m.order = append(m.order, ev.Host)
		}
		switch {
		case ev.Attempt > 0:
			row.attempt = ev.Attempt
			row.err = ev.Err
		case ev.Level >= slog.LevelInfo:
			// the row status tracks meaningful progress messages only;
			// debug chatter stays in the log tails
			row.msg = ev.Message
			row.attempt = 0
			row.err = ev.Err
		}
		if ev.Level > row.level {
			row.level = ev.Level
		}
		return m, nil
	}

	// non-host records: persist warnings and errors and informative lines
	switch {
	case ev.Level >= slog.LevelError:
		return m, tea.Println(m.styleRed.Render(firstLine(ev.Message)))
	case ev.Level >= slog.LevelWarn:
		return m, tea.Println(m.styleYellow.Render(firstLine(ev.Message)))
	case ev.Level >= slog.LevelInfo:
		return m, tea.Println(firstLine(ev.Message))
	}
	return m, nil
}

// firstLine reduces a possibly multi-line message to its first line; content
// persisted above the live region must be single-line or the repaint garbles.
func firstLine(s string) string {
	if first, _, found := strings.Cut(s, "\n"); found {
		return first + " …"
	}
	return s
}

func (m *ttyModel) View() string {
	if m.phase == "" {
		return ""
	}

	var b strings.Builder
	elapsed := time.Since(m.phaseStart).Truncate(time.Second)
	b.WriteString(m.styleCyan.Render(spinnerFrames[m.spin%len(spinnerFrames)]))
	b.WriteString(" ")
	b.WriteString(m.phase)
	if m.phaseTotal > 0 {
		b.WriteString(m.styleDim.Render(fmt.Sprintf(" · %d/%d", m.phaseStep, m.phaseTotal)))
	}
	if elapsed >= time.Second {
		b.WriteString(m.styleDim.Render(fmt.Sprintf(" (%s)", elapsed)))
	}
	b.WriteString("\n")

	hostWidth := 0
	for _, h := range m.order {
		hostWidth = max(hostWidth, len(h))
	}

	depth := tailLines
	if m.peek {
		depth = peekTailLines
	}
	showTail := func(i int) bool {
		if m.peek {
			return true
		}
		if m.focus >= 0 {
			return m.focus == i
		}
		return m.tailsAll || len(m.order) <= autoTailHosts
	}

	for i, h := range m.order {
		if i >= maxRows {
			b.WriteString(m.styleDim.Render(fmt.Sprintf("  … %d more hosts", len(m.order)-maxRows)))
			b.WriteString("\n")
			break
		}
		row := m.rows[h]
		var line strings.Builder
		line.WriteString("  ")
		if m.t.interactive && len(m.order) > 1 && i < 9 {
			line.WriteString(m.styleDim.Render(fmt.Sprintf("%d ", i+1)))
		}
		name := fmt.Sprintf("%-*s", hostWidth, h)
		switch {
		case row.level >= slog.LevelError:
			line.WriteString(m.styleRed.Render(name))
		case row.level >= slog.LevelWarn:
			line.WriteString(m.styleYellow.Render(name))
		default:
			line.WriteString(name)
		}
		line.WriteString("  ")
		if row.msg != "" {
			line.WriteString(row.msg)
		} else if last := m.t.st.rings.tail(h, 1); len(last) > 0 {
			// no meaningful status yet: show the latest log activity dimmed
			line.WriteString(m.styleDim.Render(last[0].Message))
		}
		if row.attempt > 0 {
			line.WriteString(m.styleYellow.Render(fmt.Sprintf(" ⟳ %d", row.attempt)))
			if row.err != "" {
				line.WriteString(m.styleDim.Render(fmt.Sprintf(" (%s)", row.err)))
			}
		} else if row.err != "" {
			line.WriteString(m.styleRed.Render(fmt.Sprintf(" (%s)", row.err)))
		}
		b.WriteString(ansi.Truncate(line.String(), m.width, "…"))
		b.WriteString("\n")

		if showTail(i) {
			for _, tev := range m.t.st.rings.tail(h, depth) {
				b.WriteString(ansi.Truncate(m.styleDim.Render("      │ "+tev.line()), m.width, "…"))
				b.WriteString("\n")
			}
		}
	}

	if m.t.interactive && len(m.order) > 0 {
		b.WriteString(m.styleDim.Render("  keys: space peek · 1-9 focus host logs · l all logs · 0 hide · ctrl-c abort"))
		b.WriteString("\n")
	}

	return b.String()
}
