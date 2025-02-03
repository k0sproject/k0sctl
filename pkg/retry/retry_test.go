package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	oldInterval := Interval
	Interval = 1 * time.Millisecond
	defer func() { Interval = oldInterval }()
	m.Run()
}

func TestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("succeeds on first try", func(t *testing.T) {
		err := Context(ctx, func(_ context.Context) error {
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("fails when context is canceled between tries", func(t *testing.T) {
		var counter int
		err := Context(ctx, func(_ context.Context) error {
			counter++
			if counter == 2 {
				cancel()
			}
			return errors.New("some error")
		})
		assert.Error(t, err, "foo")
	})

	t.Run("fails with a canceled context", func(t *testing.T) {
		err := Context(ctx, func(_ context.Context) error {
			return errors.New("some error")
		})
		assert.Error(t, err, "some error")
	})
}

func TestTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("succeeds before timeout", func(t *testing.T) {
		err := Timeout(ctx, 10*time.Second, func(_ context.Context) error {
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("fails on timeout", func(t *testing.T) {
		err := Timeout(ctx, 1*time.Millisecond, func(_ context.Context) error {
			time.Sleep(2 * time.Millisecond)
			return errors.New("some error")
		})
		assert.Error(t, err, "foo")
	})

	t.Run("stops retrying on ErrAbort", func(t *testing.T) {
		var counter int
		err := Timeout(ctx, 10*time.Second, func(_ context.Context) error {
			counter++
			if counter == 2 {
				return errors.Join(ErrAbort, errors.New("some error"))
			}
			return errors.New("some error")
		})
		assert.Error(t, err, "foo")
	})
}

func TestTimes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("succeeds within limit", func(t *testing.T) {
		counter := 0
		err := Times(ctx, 3, func(_ context.Context) error {
			counter++
			if counter == 2 {
				return nil
			}
			return errors.New("some error")
		})
		require.NoError(t, err)
		assert.Equal(t, 2, counter)
	})

	t.Run("fails on reaching limit", func(t *testing.T) {
		var tries int
		err := Times(ctx, 2, func(_ context.Context) error {
			tries++
			return errors.New("some error")
		})
		assert.Error(t, err, "foo")
		assert.Equal(t, 2, tries)
	})

	t.Run("stops retrying on ErrAbort", func(t *testing.T) {
		var tries int
		err := Times(ctx, 2, func(_ context.Context) error {
			tries++
			return errors.Join(ErrAbort, errors.New("some error"))
		})
		assert.Error(t, err, "foo")
		assert.Equal(t, 1, tries)
	})
}

func TestAdaptiveTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("uses existing timeout if present", func(t *testing.T) {
		parentCtx, parentCancel := context.WithTimeout(ctx, 10*time.Millisecond)
		defer parentCancel()

		start := time.Now()
		err := AdaptiveTimeout(parentCtx, 50*time.Millisecond, func(_ context.Context) error {
			time.Sleep(20 * time.Millisecond) // Should be cut off by parent timeout
			return errors.New("some error")
		})
		elapsed := time.Since(start)

		assert.Error(t, err, "some error")
		assert.Less(t, elapsed.Milliseconds(), int64(50), "should use parent timeout")
	})

	t.Run("applies new timeout if no parent timeout exists", func(t *testing.T) {
		start := time.Now()
		err := AdaptiveTimeout(ctx, 10*time.Millisecond, func(_ context.Context) error {
			time.Sleep(20 * time.Millisecond) // Should be cut off by the new timeout
			return errors.New("some error")
		})
		elapsed := time.Since(start)

		assert.Error(t, err, "some error")
		assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(10), "should use new timeout")
	})

	t.Run("uses the earlier deadline if both parent and new timeout exist", func(t *testing.T) {
		parentCtx, parentCancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer parentCancel()

		start := time.Now()
		err := AdaptiveTimeout(parentCtx, 50*time.Millisecond, func(_ context.Context) error {
			time.Sleep(30 * time.Millisecond) // Should be cut off by the parent context
			return errors.New("some error")
		})
		elapsed := time.Since(start)

		assert.Error(t, err, "some error")
		assert.Less(t, elapsed.Milliseconds(), int64(50), "should use parent timeout of 20ms")
	})

	t.Run("succeeds before timeout", func(t *testing.T) {
		err := AdaptiveTimeout(ctx, 10*time.Second, func(_ context.Context) error {
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("stops retrying on ErrAbort", func(t *testing.T) {
		var counter int
		err := AdaptiveTimeout(ctx, 10*time.Second, func(_ context.Context) error {
			counter++
			return errors.Join(ErrAbort, errors.New("some error"))
		})
		assert.Error(t, err, "some error")
		assert.Equal(t, 1, counter, "should stop retrying on ErrAbort")
	})
}
