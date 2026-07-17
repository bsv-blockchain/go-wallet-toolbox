package actions

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// withShrunkFundingBackoff temporarily reduces the package-level fundingRetryBaseBackoff so
// retry tests don't spend real wall-clock time waiting between attempts, keeping this file's
// total run time well under 1s. It restores the original value on test cleanup.
func withShrunkFundingBackoff(t *testing.T) {
	t.Helper()
	original := fundingRetryBaseBackoff
	fundingRetryBaseBackoff = time.Millisecond
	t.Cleanup(func() {
		fundingRetryBaseBackoff = original
	})
}

// TestRetryOnContention_SucceedsAfterTransientContention covers case (a): contention,
// contention, then success must result in exactly 3 closure executions and a nil error.
func TestRetryOnContention_SucceedsAfterTransientContention(t *testing.T) {
	withShrunkFundingBackoff(t)

	var calls int
	err := retryOnContention(t.Context(), logging.NewTestLogger(t), randomizer.NewTestRandomizer(), func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("attempt %d failed: %w", calls, wdk.ErrUTXOContention)
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 3, calls, "expected exactly 3 closure executions: contention, contention, success")
}

// TestRetryOnContention_ExhaustsAttemptsOnPersistentContention covers case (b): contention on
// every attempt must stop after maxFundingAttempts and return an error matching the wdk sentinel.
func TestRetryOnContention_ExhaustsAttemptsOnPersistentContention(t *testing.T) {
	withShrunkFundingBackoff(t)

	var calls int
	err := retryOnContention(t.Context(), logging.NewTestLogger(t), randomizer.NewTestRandomizer(), func() error {
		calls++
		return fmt.Errorf("attempt %d failed: %w", calls, wdk.ErrUTXOContention)
	})

	require.Error(t, err)
	require.ErrorIs(t, err, wdk.ErrUTXOContention)
	require.Equal(t, maxFundingAttempts, calls, "expected exactly maxFundingAttempts closure executions")
}

// TestRetryOnContention_NonContentionErrorDoesNotRetry covers case (c): an error that does not
// wrap wdk.ErrUTXOContention must not be retried at all.
func TestRetryOnContention_NonContentionErrorDoesNotRetry(t *testing.T) {
	withShrunkFundingBackoff(t)

	errBoom := errors.New("some other funding failure")

	var calls int
	err := retryOnContention(t.Context(), logging.NewTestLogger(t), randomizer.NewTestRandomizer(), func() error {
		calls++
		return errBoom
	})

	require.Error(t, err)
	require.ErrorIs(t, err, errBoom)
	require.False(t, errors.Is(err, wdk.ErrUTXOContention))
	require.Equal(t, 1, calls, "non-contention error must not be retried")
}

// TestRetryOnContention_AbortsOnContextCancellation exercises the ctx.Done() branch of the
// retry loop's backoff select: a canceled context must abort the wait immediately (rather than
// sleeping out the full backoff) and surface context.Canceled.
func TestRetryOnContention_AbortsOnContextCancellation(t *testing.T) {
	withShrunkFundingBackoff(t)
	// deliberately long so the test would hang if cancellation were not honored.
	fundingRetryBaseBackoff = time.Hour

	ctx, cancel := context.WithCancel(t.Context())

	errCh := make(chan error, 1)
	go func() {
		errCh <- retryOnContention(ctx, logging.NewTestLogger(t), randomizer.NewTestRandomizer(), func() error {
			return wdk.ErrUTXOContention
		})
	}()

	cancel()

	select {
	case err := <-errCh:
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("retryOnContention did not return promptly after context cancellation")
	}
}

// TestBackoffWithJitter_WithinExpectedBounds asserts the +/-50% jitter window: with a
// randomizer whose Uint64 always returns 0 (TestRandomizer), the result must be exactly the
// lower bound of the window, base/2.
func TestBackoffWithJitter_WithinExpectedBounds(t *testing.T) {
	withShrunkFundingBackoff(t)
	fundingRetryBaseBackoff = 25 * time.Millisecond

	d := backoffWithJitter(2, randomizer.NewTestRandomizer())
	base := fundingRetryBaseBackoff * 2
	require.Equal(t, base/2, d)
}
