package dbretry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
)

// deadlock mimics what reaches Do in production: the pgconn error wrapped by the repository and
// then by the action layer.
func deadlock() error {
	return fmt.Errorf("failed to update single tx after background broadcast: %w",
		fmt.Errorf("failed to update known tx status: %w",
			&pgconn.PgError{Code: deadlockDetected, Message: "deadlock detected"}))
}

func TestRetryable(t *testing.T) {
	tests := map[string]struct {
		err  error
		want bool
	}{
		"nil":                              {err: nil, want: false},
		"wrapped deadlock (40P01)":         {err: deadlock(), want: true},
		"serialization failure (40001)":    {err: &pgconn.PgError{Code: serializationFailure}, want: true},
		"unique violation is not retried":  {err: &pgconn.PgError{Code: "23505"}, want: false},
		"plain application error":          {err: errors.New("status update skipped"), want: false},
		"non-postgres engine never counts": {err: errors.New("database is locked"), want: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.want, Retryable(test.err))
		})
	}
}

func TestDo(t *testing.T) {
	original := baseBackoff
	baseBackoff = time.Millisecond
	t.Cleanup(func() { baseBackoff = original })

	random := randomizer.New()

	t.Run("returns immediately on success without retrying", func(t *testing.T) {
		calls := 0
		err := Do(context.Background(), logging.NewTestLogger(t), random, func() error {
			calls++
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 1, calls)
	})

	t.Run("re-runs the unit of work after a deadlock and succeeds", func(t *testing.T) {
		calls := 0
		err := Do(context.Background(), logging.NewTestLogger(t), random, func() error {
			calls++
			if calls == 1 {
				return deadlock()
			}
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 2, calls)
	})

	t.Run("gives up after MaxAttempts and surfaces the original error", func(t *testing.T) {
		calls := 0
		err := Do(context.Background(), logging.NewTestLogger(t), random, func() error {
			calls++
			return deadlock()
		})

		require.Error(t, err)
		assert.Equal(t, MaxAttempts, calls)
		assert.True(t, Retryable(err))

		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		assert.Equal(t, deadlockDetected, pgErr.Code)
	})

	t.Run("does not retry an application error", func(t *testing.T) {
		calls := 0
		sentinel := errors.New("status update skipped")
		err := Do(context.Background(), logging.NewTestLogger(t), random, func() error {
			calls++
			return sentinel
		})

		require.ErrorIs(t, err, sentinel)
		assert.Equal(t, 1, calls)
	})

	t.Run("stops retrying when the context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		err := Do(ctx, logging.NewTestLogger(t), random, func() error {
			calls++
			cancel()
			return deadlock()
		})

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, calls, "must not start another attempt once the context is done")
	})
}

func TestPolicy(t *testing.T) {
	original := baseBackoff
	baseBackoff = time.Millisecond
	t.Cleanup(func() { baseBackoff = original })

	t.Run("Retrying re-runs a rolled-back transaction", func(t *testing.T) {
		calls := 0
		err := Retrying(logging.NewTestLogger(t), randomizer.New()).Do(context.Background(), func() error {
			calls++
			if calls == 1 {
				return deadlock()
			}
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 2, calls)
	})

	// The Database handle passes no randomizer of its own and must still get jittered backoff.
	t.Run("Retrying falls back to a default randomizer", func(t *testing.T) {
		calls := 0
		err := Retrying(logging.NewTestLogger(t), nil).Do(context.Background(), func() error {
			calls++
			if calls == 1 {
				return deadlock()
			}
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 2, calls)
	})

	t.Run("NoRetry runs the work exactly once", func(t *testing.T) {
		calls := 0
		err := NoRetry().Do(context.Background(), func() error {
			calls++
			return deadlock()
		})

		require.Error(t, err)
		assert.Equal(t, 1, calls)
	})

	t.Run("the nil policy runs the work exactly once", func(t *testing.T) {
		var policy *Policy
		calls := 0
		err := policy.Do(context.Background(), func() error {
			calls++
			return deadlock()
		})

		require.Error(t, err)
		assert.Equal(t, 1, calls)
	})
}
