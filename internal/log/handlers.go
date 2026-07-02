package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// NewFanoutHandler returns a handler that forwards records to all given
// handlers that are enabled for the record's level.
func NewFanoutHandler(handlers ...slog.Handler) slog.Handler {
	return &fanoutHandler{handlers: handlers}
}

type fanoutHandler struct {
	handlers []slog.Handler
}

func (f *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			errs = append(errs, h.Handle(ctx, r.Clone()))
		}
	}
	return errors.Join(errs...)
}

func (f *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: next}
}

func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return &fanoutHandler{handlers: next}
}

// NewFileHandler returns a handler that writes structured logfmt records,
// used for the persistent log file.
func NewFileHandler(w io.Writer, level slog.Leveler) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				if lvl, ok := a.Value.Any().(slog.Level); ok {
					a.Value = slog.StringValue(levelName(lvl))
				}
			}
			return a
		},
	})
}

const (
	ansiReset  = "\x1b[0m"
	ansiGray   = "\x1b[90m"
	ansiCyan   = "\x1b[36m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
)

func levelColor(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return ansiGray
	case l < slog.LevelWarn:
		return ansiCyan
	case l < slog.LevelError:
		return ansiYellow
	default:
		return ansiRed
	}
}

// levelTag returns the 4-letter level tag used on screen.
func levelTag(l slog.Level) string {
	return (levelName(l) + "    ")[:4]
}

// NewScreenHandler returns the handler that renders records for the terminal:
// a colored level tag, the host attribute as a message prefix, the message,
// and any remaining attributes as trailing key=value pairs. This handler is
// the seam where a richer display implementation can be plugged in later.
func NewScreenHandler(w io.Writer, level slog.Leveler, colors bool) slog.Handler {
	return &screenHandler{w: w, level: level, colors: colors, mu: &sync.Mutex{}}
}

type screenHandler struct {
	w      io.Writer
	level  slog.Leveler
	colors bool
	mu     *sync.Mutex
	attrs  []slog.Attr
}

func (s *screenHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= s.level.Level()
}

func (s *screenHandler) Handle(_ context.Context, r slog.Record) error {
	var host, phase string
	var rest []slog.Attr

	collect := func(a slog.Attr) {
		a.Value = a.Value.Resolve()
		if a.Key == KeyHost && host == "" {
			host = a.Value.String()
			return
		}
		// phase attr is consumed by the banner rendering on info level;
		// on other levels (e.g. debug "phase completed") it stays visible
		if a.Key == KeyPhase && phase == "" && r.Level == slog.LevelInfo {
			phase = a.Value.String()
			return
		}
		if a.Key == KeyError && a.Value.String() == "" {
			return
		}
		rest = append(rest, a)
	}
	for _, a := range s.attrs {
		collect(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		collect(a)
		return true
	})

	var b strings.Builder
	color := ""
	reset := ""
	if s.colors {
		color = levelColor(r.Level)
		reset = ansiReset
	}
	b.WriteString(color)
	b.WriteString(levelTag(r.Level))
	b.WriteString(reset)
	b.WriteString(" ")
	if host != "" {
		b.WriteString(host)
		b.WriteString(": ")
	}
	// phase banners (info-level records carrying the phase attr) render green
	if phase != "" && r.Level == slog.LevelInfo && s.colors {
		b.WriteString(ansiGreen)
		b.WriteString(r.Message)
		b.WriteString(ansiReset)
	} else {
		b.WriteString(r.Message)
	}
	for _, a := range rest {
		b.WriteString(" ")
		b.WriteString(color)
		b.WriteString(a.Key)
		b.WriteString(reset)
		fmt.Fprintf(&b, "=%q", a.Value.String())
	}
	b.WriteString("\n")

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := io.WriteString(s.w, b.String())
	return err
}

func (s *screenHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *s
	next.attrs = make([]slog.Attr, 0, len(s.attrs)+len(attrs))
	next.attrs = append(next.attrs, s.attrs...)
	next.attrs = append(next.attrs, attrs...)
	return &next
}

func (s *screenHandler) WithGroup(_ string) slog.Handler {
	// k0sctl doesn't use attr groups; flatten them.
	return s
}
