package retry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	log "github.com/k0sproject/k0sctl/internal/log"
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
	t.Run("succeeds before timeout", func(t *testing.T) {
		err := Timeout(context.Background(), 10*time.Second, func(_ context.Context) error {
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("fails on timeout", func(t *testing.T) {
		err := Timeout(context.Background(), 1*time.Millisecond, func(_ context.Context) error {
			time.Sleep(2 * time.Millisecond)
			return errors.New("some error")
		})
		assert.Error(t, err, "foo")
	})

	t.Run("stops retrying on ErrAbort", func(t *testing.T) {
		var counter int
		err := Timeout(context.Background(), 10*time.Second, func(_ context.Context) error {
			counter++
			if counter == 2 {
				return errors.Join(ErrAbort, errors.New("some error"))
			}
			return errors.New("some error")
		})
		assert.Error(t, err, "foo")
	})

	t.Run("respects parent deadline", func(t *testing.T) {
		parentCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := Timeout(parentCtx, 50*time.Millisecond, func(child context.Context) error {
			<-child.Done()
			return child.Err()
		})
		elapsed := time.Since(start)

		assert.Error(t, err) //nolint:testifylint
		assert.Less(t, elapsed, 50*time.Millisecond)
	})

	t.Run("applies new timeout when parent has none", func(t *testing.T) {
		start := time.Now()
		err := Timeout(context.Background(), 10*time.Millisecond, func(_ context.Context) error {
			time.Sleep(20 * time.Millisecond)
			return errors.New("some error")
		})
		elapsed := time.Since(start)

		assert.Error(t, err) //nolint:testifylint
		assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(10))
	})

	t.Run("does not add deadline when timeout disabled", func(t *testing.T) {
		var deadlineSet bool
		err := Timeout(context.Background(), 0, func(ctx context.Context) error {
			_, deadlineSet = ctx.Deadline()
			return nil
		})
		require.NoError(t, err)
		assert.False(t, deadlineSet)
	})
}

func TestTimes(t *testing.T) {
	ctx := t.Context()

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
		assert.Error(t, err, "foo") //nolint:testifylint
		assert.Equal(t, 2, tries)
	})

	t.Run("stops retrying on ErrAbort", func(t *testing.T) {
		var tries int
		err := Times(ctx, 2, func(_ context.Context) error {
			tries++
			return errors.Join(ErrAbort, errors.New("some error"))
		})
		assert.Error(t, err, "foo") //nolint:testifylint
		assert.Equal(t, 1, tries)
	})
}

func TestWithDefaultTimeout(t *testing.T) {
	ctx := t.Context()

	old := DefaultTimeout
	DefaultTimeout = 5 * time.Millisecond
	defer func() { DefaultTimeout = old }()

	start := time.Now()
	err := WithDefaultTimeout(ctx, func(_ context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return errors.New("fail")
	})
	elapsed := time.Since(start)

	assert.Error(t, err) //nolint:testifylint
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(5))
}

// retryLogRecord is a flattened view of a captured slog.Record, pulling out
// the attrs logAttempt attaches so tests can assert on them without
// depending on any particular rendering.
type retryLogRecord struct {
	level      slog.Level
	message    string
	attempt    int64
	hasAttempt bool
	errText    string
	host       string
}

// capturingHandler is a minimal slog.Handler that records every record it
// receives, merging in attrs attached via WithAttrs the way a real handler
// would, so tests can inspect what retry logged without parsing text output.
type capturingHandler struct {
	mu      *sync.Mutex
	records *[]retryLogRecord
	attrs   []slog.Attr
}

func newCapturingHandler() *capturingHandler {
	return &capturingHandler{mu: &sync.Mutex{}, records: &[]retryLogRecord{}}
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	rec := retryLogRecord{level: r.Level, message: r.Message}
	apply := func(a slog.Attr) {
		switch a.Key {
		case "attempt":
			rec.attempt = a.Value.Int64()
			rec.hasAttempt = true
		case log.KeyError:
			rec.errText = a.Value.String()
		case log.KeyHost:
			rec.host = a.Value.String()
		}
	}
	for _, a := range h.attrs {
		apply(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		apply(a)
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, rec)
	return nil
}

func (h *capturingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *capturingHandler) WithGroup(_ string) slog.Handler { return h }

func (h *capturingHandler) Records() []retryLogRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]retryLogRecord, len(*h.records))
	copy(out, *h.records)
	return out
}

// retryingRecords filters out the Tracef bookkeeping records emitted by
// Context/Times so tests only see the ones logAttempt produced.
func retryingRecords(recs []retryLogRecord) []retryLogRecord {
	var out []retryLogRecord
	for _, r := range recs {
		if r.message == "retrying" {
			out = append(out, r)
		}
	}
	return out
}

func installCapturingHandler(t *testing.T) *capturingHandler {
	t.Helper()
	orig := log.Base()
	t.Cleanup(func() { log.SetLogger(orig) })

	h := newCapturingHandler()
	log.SetLogger(slog.New(h))
	return h
}

func TestLogAttemptRoutesThroughContextLogger(t *testing.T) {
	h := installCapturingHandler(t)

	scoped := log.With(log.KeyHost, "node-a")
	ctx := log.IntoContext(context.Background(), scoped)

	logAttempt(ctx, 1, errors.New("boom"))
	logAttempt(ctx, 3, errors.New("boom again"))

	recs := h.Records()
	require.Len(t, recs, 2)

	assert.Equal(t, slog.LevelDebug, recs[0].level, "attempt 1 should log at debug")
	assert.Equal(t, "retrying", recs[0].message)
	assert.True(t, recs[0].hasAttempt)
	assert.Equal(t, int64(1), recs[0].attempt)
	assert.Equal(t, "node-a", recs[0].host)
	assert.Equal(t, "boom", recs[0].errText)

	assert.Equal(t, slog.LevelInfo, recs[1].level, "attempt 3 should log at info")
	assert.Equal(t, int64(3), recs[1].attempt)
	assert.Equal(t, "boom again", recs[1].errText)
}

func TestLogAttemptFallsBackToBaseLoggerWithoutContextLogger(t *testing.T) {
	h := installCapturingHandler(t)

	logAttempt(context.Background(), 1, errors.New("boom"))

	recs := h.Records()
	require.Len(t, recs, 1)
	assert.Empty(t, recs[0].host, "no host should be attached without a scoped context logger")
}

func TestLogAttemptLevelPolicy(t *testing.T) {
	orig := log.Base()
	t.Cleanup(func() { log.SetLogger(orig) })

	tests := []struct {
		attempt int
		want    slog.Level
	}{
		{1, slog.LevelDebug},
		{2, slog.LevelDebug},
		{3, slog.LevelInfo},
		{4, slog.LevelDebug},
		{8, slog.LevelDebug},
		{9, slog.LevelInfo},
		{10, slog.LevelDebug},
		{15, slog.LevelInfo},
		{16, slog.LevelDebug},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt=%d", tt.attempt), func(t *testing.T) {
			h := newCapturingHandler()
			log.SetLogger(slog.New(h))

			logAttempt(context.Background(), tt.attempt, errors.New("fail"))

			recs := h.Records()
			require.Len(t, recs, 1)
			assert.Equal(t, tt.want, recs[0].level, "logAttempt(attempt=%d) level", tt.attempt)
		})
	}
}

func TestRetryTimesLogsAttemptsThroughLogAttempt(t *testing.T) {
	h := installCapturingHandler(t)

	ctx := t.Context()
	err := Times(ctx, 4, func(_ context.Context) error {
		return errors.New("still failing")
	})
	require.Error(t, err)

	recs := retryingRecords(h.Records())
	require.Len(t, recs, 3, "expected one retrying log per retry tick before the attempt limit was hit")

	wantAttempts := []int64{2, 3, 4}
	wantLevels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelDebug}
	for i, rec := range recs {
		assert.Equal(t, wantAttempts[i], rec.attempt, "record %d attempt", i)
		assert.Equal(t, wantLevels[i], rec.level, "record %d level", i)
		assert.Equal(t, "still failing", rec.errText, "record %d error text", i)
	}
}

func TestRetryContextLogsAttemptsThroughLogAttempt(t *testing.T) {
	h := installCapturingHandler(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var calls int
	err := Context(ctx, func(_ context.Context) error {
		calls++
		if calls == 4 {
			cancel()
		}
		return errors.New("still failing")
	})
	require.Error(t, err)

	recs := retryingRecords(h.Records())
	require.Len(t, recs, 3, "expected one retrying log per retry tick before cancellation")

	wantAttempts := []int64{1, 2, 3}
	wantLevels := []slog.Level{slog.LevelDebug, slog.LevelDebug, slog.LevelInfo}
	for i, rec := range recs {
		assert.Equal(t, wantAttempts[i], rec.attempt, "record %d attempt", i)
		assert.Equal(t, wantLevels[i], rec.level, "record %d level", i)
		assert.Equal(t, "still failing", rec.errText, "record %d error text", i)
	}
}
