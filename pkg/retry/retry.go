// Package retry provides simple retry wrappers for functions that return an error.
// Internally delegates to github.com/k0sproject/rig/v2/retry.
//
// Behavioral note: the delay between attempts grows linearly (delay × attempt)
// due to rig's default backoff model. The constant-interval behaviour of the
// previous implementation is not preserved, but the effective retry window is
// equivalent when Interval is small relative to the timeout.
package retry

import (
	"context"
	"errors"
	"time"

	rigretry "github.com/k0sproject/rig/v2/retry"
)

var (
	// DefaultTimeout is a default timeout for retry operations.
	DefaultTimeout = 2 * time.Minute
	// Interval is the base delay between retry attempts.
	Interval = 5 * time.Second
	// ErrAbort should be returned when an error occurs on which retrying should be aborted.
	ErrAbort = errors.New("retrying aborted")
)

// notAbort returns false (stop retrying) when the error wraps ErrAbort.
func notAbort(err error) bool {
	return !errors.Is(err, ErrAbort)
}

// Context retries f until it succeeds or the context is cancelled.
func Context(ctx context.Context, f func(ctx context.Context) error) error {
	return rigretry.DoWithContext(ctx, f,
		rigretry.Delay(Interval),
		rigretry.If(notAbort),
	)
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
	return rigretry.DoWithContext(ctx, f,
		rigretry.Delay(Interval),
		rigretry.MaxRetries(times),
		rigretry.If(notAbort),
	)
}
