package display

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	log "github.com/k0sproject/k0sctl/internal/log"
)

// dumpLines is how many recent per-host log lines are shown on failure.
const dumpLines = 15

// Display is the slog.Handler that renders k0sctl's screen output. It keeps
// per-host ring buffers of recent records regardless of the visible level so
// that failures can be explained, and delegates visible records either to a
// plain line renderer or to a live TTY renderer.
type Display struct {
	st *state
	// attrs mirrors handler-scoped attributes (attached via Logger.With)
	// so parseEvent sees them alongside the record's own attributes.
	attrs []slog.Attr
	// screen is the plain line renderer carrying the same scoped attrs.
	screen slog.Handler
}

type state struct {
	rings *rings
	out   io.Writer
	level slog.Leveler
	tty   *ttyRenderer

	mu     sync.Mutex
	dumped bool
}

// NewPlain returns a Display that renders classic line output. Used for
// non-TTY output (pipes, CI), --debug/--trace, and as the fallback renderer.
func NewPlain(out io.Writer, level slog.Leveler, colors bool) *Display {
	return &Display{
		st:     &state{rings: newRings(), out: out, level: level},
		screen: log.NewScreenHandler(out, level, colors),
	}
}

// NewTTY returns a Display that renders a live progress view on the given
// terminal writer. interactive enables keyboard input (stdin is a TTY);
// onInterrupt is called when the user presses ctrl-c, and should cancel the
// ongoing operation.
func NewTTY(out io.Writer, level slog.Leveler, interactive bool, onInterrupt func()) *Display {
	st := &state{rings: newRings(), out: out, level: level}
	st.tty = newTTYRenderer(st, out, interactive, onInterrupt)
	return &Display{
		st:     st,
		screen: log.NewScreenHandler(out, level, true),
	}
}

// Stop finalizes the display. For the TTY renderer this shuts down the live
// view and restores the terminal. Safe to call multiple times and on plain
// displays.
func (d *Display) Stop() {
	if d.st.tty != nil {
		d.st.tty.stop()
	}
}

// Enabled implements slog.Handler. The display always wants records: the
// ring buffers capture below-visible-level records for failure forensics.
func (d *Display) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle implements slog.Handler.
func (d *Display) Handle(ctx context.Context, r slog.Record) error {
	ev := parseEvent(d.attrs, r)
	d.st.rings.add(ev)

	// a fatal record ends the run: explain failed hosts before the final
	// message, in both plain and TTY modes
	if r.Level >= log.LevelFatal {
		if d.st.tty != nil {
			d.st.tty.stop()
		}
		d.st.dumpFailures()
	}

	if d.st.tty != nil && d.st.tty.running() {
		// the live view starts on the first phase record: everything before
		// it (logo, banners, config parsing) is direct terminal output that
		// must complete before bubbletea switches the terminal to raw mode
		if d.st.tty.hasStarted() || ev.Phase != "" {
			d.st.tty.send(ev)
			return nil
		}
	}

	if d.screen.Enabled(ctx, r.Level) {
		return d.screen.Handle(ctx, r)
	}
	return nil
}

// WithAttrs implements slog.Handler.
func (d *Display) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Attr, 0, len(d.attrs)+len(attrs))
	next = append(next, d.attrs...)
	next = append(next, attrs...)
	return &Display{st: d.st, attrs: next, screen: d.screen.WithAttrs(attrs)}
}

// WithGroup implements slog.Handler. k0sctl doesn't use attr groups; flatten.
func (d *Display) WithGroup(_ string) slog.Handler {
	return d
}

// dumpFailures prints the recent log tail of every host that reported an
// error. It runs at most once per display.
func (st *state) dumpFailures() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.dumped {
		return
	}
	st.dumped = true

	for host, events := range st.rings.failures(dumpLines) {
		fmt.Fprintf(st.out, "\nlast %d log entries for host %s:\n", len(events), host)
		for _, ev := range events {
			fmt.Fprintf(st.out, "  %s\n", ev.line())
		}
	}
}
