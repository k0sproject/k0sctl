// Package retry provides simple retry helpers for functions returning an error.
package retry

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/k0sproject/k0sctl/internal/log"
)

var (
	// DefaultTimeout is a default timeout for retry operations.
	DefaultTimeout = 2 * time.Minute
	// Interval is the time to wait between retry attempts.
	Interval = 5 * time.Second
	// ErrAbort should be returned when an error occurs on which retrying should be aborted.
	ErrAbort = errors.New("retrying aborted")
)

// logAttempt logs a retry attempt with structured attributes using the logger
// carried by the context, so the host being retried against is attached when
// the retry happens inside a per-host operation. To keep the default output
// calm during normal short waits, attempts only surface at info level once
// ~15 seconds have passed and then roughly twice a minute; the rest go to
// debug.
func logAttempt(ctx context.Context, attempt int, lastErr error) {
	logger := log.FromContext(ctx).With("attempt", attempt, log.KeyError, lastErr.Error())
	if attempt >= 3 && attempt%6 == 3 {
		logger.Info("retrying")
	} else {
		logger.Debug("retrying")
	}
}

// Context retries f at constant Interval until it succeeds or the context is cancelled.
func Context(ctx context.Context, f func(ctx context.Context) error) error {
	var lastErr error

	if ctx.Err() != nil {
		return ctx.Err()
	}

	lastErr = f(ctx)
	if lastErr == nil || errors.Is(lastErr, ErrAbort) {
		return lastErr
	}

	ticker := time.NewTicker(Interval)
	defer ticker.Stop()

	attempt := 0
	for {
		select {
		case <-ctx.Done():
			log.FromContext(ctx).Tracef("retry.Context: context cancelled after %d attempts", attempt)
			return errors.Join(ctx.Err(), lastErr)
		case <-ticker.C:
			attempt++
			if lastErr != nil {
				logAttempt(ctx, attempt, lastErr)
			}
			lastErr = f(ctx)
			if errors.Is(lastErr, ErrAbort) {
				log.FromContext(ctx).Tracef("retry.Context: aborted after %d attempts", attempt)
				return lastErr
			}
			if lastErr == nil {
				log.FromContext(ctx).Tracef("retry.Context: succeeded after %d attempts", attempt)
				return nil
			}
			log.FromContext(ctx).Tracef("retry.Context: attempt %d failed: %s", attempt, lastErr)
		}
	}
}

// Timeout retries f until it succeeds, the context is canceled, or the timeout
// is reached. If timeout <= 0, no additional deadline is set.
func Timeout(ctx context.Context, timeout time.Duration, f func(ctx context.Context) error) error {
	var (
		child  context.Context
		cancel context.CancelFunc
	)
	if timeout <= 0 {
		child, cancel = context.WithCancel(ctx)
	} else {
		child, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	return Context(child, f)
}

// WithDefaultTimeout wraps f with Timeout using DefaultTimeout.
func WithDefaultTimeout(ctx context.Context, f func(ctx context.Context) error) error {
	return Timeout(ctx, DefaultTimeout, f)
}

// Times retries f until it succeeds or the given number of attempts have been made.
func Times(ctx context.Context, times int, f func(context.Context) error) error {
	var lastErr error

	lastErr = f(ctx)
	if lastErr == nil || errors.Is(lastErr, ErrAbort) {
		return lastErr
	}

	i := 1
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.FromContext(ctx).Tracef("retry.Times: context cancelled after %d attempts", i)
			return errors.Join(ctx.Err(), lastErr)
		case <-ticker.C:
			if lastErr != nil {
				logAttempt(ctx, i+1, lastErr)
			}
			lastErr = f(ctx)
			if errors.Is(lastErr, ErrAbort) {
				log.FromContext(ctx).Tracef("retry.Times: aborted after %d attempts", i)
				return lastErr
			}
			if lastErr == nil {
				log.FromContext(ctx).Tracef("retry.Times: succeeded on attempt %d", i)
				return nil
			}
			i++
			if i >= times {
				log.FromContext(ctx).Tracef("retry.Times: exceeded %d attempts", times)
				return fmt.Errorf("retry limit exceeded after %d attempts: %w", times, lastErr)
			}
		}
	}
}
