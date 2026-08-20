// Package dbretry re-runs a database transaction the engine rolled back to break a conflict
// between concurrent sessions. Such a rollback is not an application error - the same work run
// again normally commits. Everything else is returned to the caller untouched.
package dbretry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Postgres class-40 (transaction rollback) SQLSTATEs: the server aborted the transaction to
// resolve a conflict with a concurrent session, and expects the client to retry.
const (
	deadlockDetected     = "40P01"
	serializationFailure = "40001"
)

// MaxAttempts bounds the re-runs before the error is returned. The victim's rollback frees the
// locks immediately, so one retry almost always suffices; beyond three, the cause is a
// systematic lock-order problem that should be seen rather than hidden.
const MaxAttempts = 3

// baseBackoff is the delay before the first retry. A var, not a const, so tests can shrink it.
var baseBackoff = 20 * time.Millisecond

// Retryable reports whether err is - or wraps - a Postgres class-40 transaction rollback.
// Deliberately narrow: an application error, a constraint violation or a failed CAS re-runs to
// the same outcome, so retrying it only holds a connection open.
func Retryable(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == deadlockDetected || pgErr.Code == serializationFailure
}

// Do runs fn while it returns a Retryable error, up to MaxAttempts times, waiting a jittered
// backoff in between and returning early if ctx is canceled. A conflict that survives every
// attempt surfaces with its original SQLSTATE.
//
// fn MUST be safe to run more than once: the engine rolled the failed attempt back before Do
// saw the error, so a caller that only writes to the database qualifies.
func Do(ctx context.Context, logger *slog.Logger, random wdk.Randomizer, fn func() error) error {
	for attempt := 1; ; attempt++ {
		err := fn()
		if err == nil || !Retryable(err) || attempt == MaxAttempts {
			return err
		}

		logger.WarnContext(ctx, "retrying database transaction after a transient rollback",
			slog.Int("attempt", attempt), slog.Int("maxAttempts", MaxAttempts), logging.Error(err))

		select {
		case <-ctx.Done():
			return fmt.Errorf("database retry aborted: %w", ctx.Err())
		case <-time.After(backoffWithJitter(attempt, random)):
		}
	}
}

// backoffWithJitter returns baseBackoff*attempt with +/-50% jitter, so transactions that just
// deadlocked do not line up to collide again.
func backoffWithJitter(attempt int, random wdk.Randomizer) time.Duration {
	base := baseBackoff * time.Duration(attempt)
	if base <= 0 {
		return 0
	}

	jitter := random.Uint64(uint64(base))

	return base/2 + time.Duration(jitter) //nolint:gosec // jitter < base <= MaxInt64, conversion cannot overflow
}

// Policy is a pre-bound decision about whether a database handle may retry the transactions it
// opens. The same repository code runs against two kinds of handle, and only one may retry.
type Policy struct {
	logger *slog.Logger
	random wdk.Randomizer
}

// Retrying returns a Policy for a handle that owns the transactions it opens - the top-level
// connection. A nil random selects a default one; pass your own to make the jitter deterministic.
func Retrying(logger *slog.Logger, random wdk.Randomizer) *Policy {
	if random == nil {
		random = randomizer.New()
	}

	return &Policy{logger: logger, random: random}
}

// NoRetry returns a Policy that runs the work exactly once.
//
// Required for a handle bound to an outer transaction: gorm makes that a SAVEPOINT, and rolling
// back to it frees only the locks taken since it opened, not the outer ones the deadlock cycle
// runs through. Only the owner of the outer transaction can resolve it, and it retries.
//
// Also the honest choice where there is no concurrency to retry against, such as a test.
func NoRetry() *Policy {
	return &Policy{}
}

// Do runs fn under the policy. The zero Policy and a nil *Policy both run fn once: a handle
// never told it owns its transactions must not assume it does.
func (p *Policy) Do(ctx context.Context, fn func() error) error {
	if p == nil || p.logger == nil {
		return fn()
	}

	return Do(ctx, p.logger, p.random, fn)
}
