package consumer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestConsumer() *Consumer {
	return &Consumer{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// ─── retryInsert ─────────────────────────────────────────────────────────────

func TestRetryInsert_SuccessOnFirstAttempt(t *testing.T) {
	c := newTestConsumer()
	calls := 0

	ok := c.retryInsert(context.Background(), func() error {
		calls++
		return nil
	}, "id-1")

	assert.True(t, ok)
	assert.Equal(t, 1, calls)
}

func TestRetryInsert_SuccessOnSecondAttempt(t *testing.T) {
	c := newTestConsumer()
	calls := 0

	ok := c.retryInsert(context.Background(), func() error {
		calls++
		if calls < 2 {
			return errors.New("transient error")
		}
		return nil
	}, "id-1")

	assert.True(t, ok)
	assert.Equal(t, 2, calls)
}

func TestRetryInsert_ExhaustsAllRetries(t *testing.T) {
	c := newTestConsumer()
	calls := 0

	ok := c.retryInsert(context.Background(), func() error {
		calls++
		return errors.New("permanent error")
	}, "id-1")

	assert.False(t, ok)
	assert.Equal(t, maxRetries, calls)
}

func TestRetryInsert_CancelledContext(t *testing.T) {
	c := newTestConsumer()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	ok := c.retryInsert(ctx, func() error {
		calls++
		cancel()
		return errors.New("error")
	}, "id-1")

	assert.False(t, ok)
	assert.Equal(t, 1, calls)
}

func TestRetryInsert_ZeroWaitOnSuccess(t *testing.T) {
	c := newTestConsumer()
	start := time.Now()

	c.retryInsert(context.Background(), func() error { return nil }, "id-1")

	assert.Less(t, time.Since(start), 100*time.Millisecond)
}
