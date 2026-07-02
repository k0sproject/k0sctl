// Package log is k0sctl's logging facade: a thin printf-style API over
// log/slog with an added trace level and support for scoped loggers that
// carry structured attributes such as the host they relate to.
package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	riglog "github.com/k0sproject/rig/v2/log"
)

// Levels in addition to the standard log/slog levels.
const (
	LevelTrace = slog.LevelDebug - 4
	LevelFatal = slog.LevelError + 4
)

// Attribute keys shared with rig so that records from both sources can be
// routed and filtered uniformly, plus k0sctl's own keys for progress events.
const (
	KeyHost     = riglog.KeyHost
	KeyError    = riglog.KeyError
	KeyDuration = riglog.KeyDuration
	KeyExitCode = riglog.KeyExitCode

	// KeyPhase marks records that carry phase lifecycle information: the
	// phase manager attaches it to phase start/completion records so that
	// displays can track progress without parsing messages.
	KeyPhase = "phase"
	// KeyAttempt carries the retry attempt number on records emitted by
	// pkg/retry, letting displays render live retry counters.
	KeyAttempt = "attempt"
)

var base atomic.Pointer[slog.Logger]

func init() {
	base.Store(slog.New(NewScreenHandler(os.Stderr, slog.LevelInfo, false)))
}

// SetLogger replaces the logger used by the package level functions and
// loggers derived via With.
func SetLogger(l *slog.Logger) {
	base.Store(l)
}

// Base returns the current base logger without any attached attributes,
// suitable for injecting into libraries such as rig that tag their own
// records.
func Base() *slog.Logger {
	return base.Load()
}

// Logger is a printf-style logger bound to a set of structured attributes.
type Logger struct {
	sl *slog.Logger
}

// With returns a Logger carrying the given attributes. The arguments are
// interpreted like [slog.Logger.With]: alternating keys and values, or
// [slog.Attr] values.
func With(args ...any) *Logger {
	return &Logger{sl: Base().With(args...)}
}

// With returns a Logger carrying the receiver's attributes plus the given ones.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{sl: l.sl.With(args...)}
}

// Slog returns the underlying *slog.Logger.
func (l *Logger) Slog() *slog.Logger {
	return l.sl
}

func (l *Logger) log(level slog.Level, msg string) {
	l.sl.Log(context.Background(), level, msg)
}

func (l *Logger) Tracef(format string, args ...any) { l.log(LevelTrace, fmt.Sprintf(format, args...)) }
func (l *Logger) Debugf(format string, args ...any) {
	l.log(slog.LevelDebug, fmt.Sprintf(format, args...))
}

func (l *Logger) Infof(format string, args ...any) {
	l.log(slog.LevelInfo, fmt.Sprintf(format, args...))
}

func (l *Logger) Warnf(format string, args ...any) {
	l.log(slog.LevelWarn, fmt.Sprintf(format, args...))
}

func (l *Logger) Errorf(format string, args ...any) {
	l.log(slog.LevelError, fmt.Sprintf(format, args...))
}

func (l *Logger) Trace(args ...any) { l.log(LevelTrace, fmt.Sprint(args...)) }
func (l *Logger) Debug(args ...any) { l.log(slog.LevelDebug, fmt.Sprint(args...)) }
func (l *Logger) Info(args ...any)  { l.log(slog.LevelInfo, fmt.Sprint(args...)) }
func (l *Logger) Warn(args ...any)  { l.log(slog.LevelWarn, fmt.Sprint(args...)) }
func (l *Logger) Error(args ...any) { l.log(slog.LevelError, fmt.Sprint(args...)) }

// Fatal logs at fatal level and exits the process, like logrus Fatal did.
func (l *Logger) Fatal(args ...any) {
	l.log(LevelFatal, fmt.Sprint(args...))
	os.Exit(1)
}

// Fatalf logs at fatal level and exits the process, like logrus Fatalf did.
func (l *Logger) Fatalf(format string, args ...any) {
	l.log(LevelFatal, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func std() *Logger { return &Logger{sl: Base()} }

func Tracef(format string, args ...any) { std().Tracef(format, args...) }
func Debugf(format string, args ...any) { std().Debugf(format, args...) }
func Infof(format string, args ...any)  { std().Infof(format, args...) }
func Warnf(format string, args ...any)  { std().Warnf(format, args...) }
func Errorf(format string, args ...any) { std().Errorf(format, args...) }
func Fatalf(format string, args ...any) { std().Fatalf(format, args...) }

func Trace(args ...any) { std().Trace(args...) }
func Debug(args ...any) { std().Debug(args...) }
func Info(args ...any)  { std().Info(args...) }
func Warn(args ...any)  { std().Warn(args...) }
func Error(args ...any) { std().Error(args...) }
func Fatal(args ...any) { std().Fatal(args...) }

type ctxKey struct{}

// IntoContext returns a context carrying the given logger. Deep helpers such
// as pkg/retry use it to log with the caller's scope (e.g. the host) attached.
func IntoContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the logger carried by the context, or a logger backed
// by the base logger when the context has none.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(ctxKey{}).(*Logger); ok {
		return l
	}
	return std()
}

// levelName returns the display name for a level, covering the custom trace
// and fatal levels that slog would render as "DEBUG-4" and "ERROR+4".
func levelName(l slog.Level) string {
	switch {
	case l < slog.LevelDebug:
		return "TRACE"
	case l >= LevelFatal:
		return "FATAL"
	default:
		return l.String()
	}
}
