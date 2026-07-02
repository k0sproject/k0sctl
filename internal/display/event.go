// Package display renders k0sctl's progress for the user. It consumes the
// same slog record stream that feeds the log file: log records carry
// structured attributes (host, phase, duration, attempt) that displays use
// to route and render without parsing messages. Two modes exist: a plain
// line-based renderer for non-TTY output, --debug/--trace and CI, and a live
// TTY renderer.
package display

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	log "github.com/k0sproject/k0sctl/internal/log"
)

// Event is a parsed log record with k0sctl's routing attributes extracted.
type Event struct {
	Time     time.Time
	Level    slog.Level
	Message  string
	Host     string
	Phase    string
	Step     int64
	Total    int64
	Duration time.Duration
	Attempt  int64
	Err      string
	// Rest contains the remaining attributes, pre-rendered as "key=value".
	Rest []string
}

// parseEvent extracts routing attributes from handler-scoped attrs (attached
// via Logger.With) and the record's own attrs.
func parseEvent(attrs []slog.Attr, r slog.Record) Event {
	ev := Event{Time: r.Time, Level: r.Level, Message: r.Message}

	collect := func(a slog.Attr) {
		a.Value = a.Value.Resolve()
		switch a.Key {
		case log.KeyHost:
			if ev.Host == "" {
				ev.Host = normalizeHost(a.Value.String())
			}
		case log.KeyPhase:
			if ev.Phase == "" {
				ev.Phase = a.Value.String()
			}
		case log.KeyPhaseStep:
			if a.Value.Kind() == slog.KindInt64 {
				ev.Step = a.Value.Int64()
			}
		case log.KeyPhaseTotal:
			if a.Value.Kind() == slog.KindInt64 {
				ev.Total = a.Value.Int64()
			}
		case log.KeyDuration:
			if a.Value.Kind() == slog.KindDuration {
				ev.Duration = a.Value.Duration()
			}
		case log.KeyAttempt:
			if a.Value.Kind() == slog.KindInt64 {
				ev.Attempt = a.Value.Int64()
			}
		case log.KeyError:
			if ev.Err == "" {
				ev.Err = a.Value.String()
			}
		default:
			ev.Rest = append(ev.Rest, a.Key+"="+attrValue(a.Value.String()))
		}
	}

	for _, a := range attrs {
		collect(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		collect(a)
		return true
	})

	return ev
}

// line renders the event as a single plain-text line for log tails,
// including a one-letter level marker (T/D/I/W/E/F).
func (ev Event) line() string {
	var b strings.Builder
	b.WriteString(ev.Time.Format("15:04:05"))
	b.WriteString(" ")
	b.WriteString(levelMark(ev.Level))
	b.WriteString(" ")
	b.WriteString(ev.Message)
	if ev.Attempt > 0 {
		b.WriteString(" attempt=")
		b.WriteString(strconv.FormatInt(ev.Attempt, 10))
	}
	if ev.Err != "" {
		b.WriteString(" error=")
		b.WriteString(attrValue(ev.Err))
	}
	for _, kv := range ev.Rest {
		b.WriteString(" ")
		b.WriteString(kv)
	}
	return b.String()
}

func levelMark(l slog.Level) string {
	switch {
	case l < slog.LevelDebug:
		return "T"
	case l < slog.LevelInfo:
		return "D"
	case l < slog.LevelWarn:
		return "I"
	case l < slog.LevelError:
		return "W"
	case l < log.LevelFatal:
		return "E"
	default:
		return "F"
	}
}

// normalizeHost merges the two host identities rig uses: before a connection
// exists, rig tags records with the connection config's string, which renders
// as e.g. `ssh.Config{addr:port}`; once connected, records carry plain
// `addr:port`. k0sctl's Host.String() produces the latter, so the config
// wrapper is stripped to route both to the same host.
func normalizeHost(host string) string {
	if start := strings.Index(host, ".Config{"); start > 0 && strings.HasSuffix(host, "}") {
		if inner := host[start+len(".Config{") : len(host)-1]; inner != "" {
			return inner
		}
	}
	return host
}

// attrValueMax caps rendered attribute values in log tails; remote command
// scripts can be kilobytes of multi-line text.
const attrValueMax = 120

// attrValue makes an attribute value fit on a single tail line: control
// characters are escaped and long values truncated.
func attrValue(s string) string {
	if len(s) > attrValueMax {
		s = s[:attrValueMax] + "…"
	}
	if strings.ContainsAny(s, " \t\n\r\"") {
		return strconv.Quote(s)
	}
	return s
}
