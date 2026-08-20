package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
)

// runInTransaction runs fn in a single database transaction, re-running it when the engine
// rolls it back to break a conflict with a concurrent session. Whether that retry happens is
// decided by the policy the repository was built with, not here: the top-level connection owns
// its transactions and retries, a unit-of-work handle opens a savepoint and must not.
//
// fn must be safe to run more than once, which rules out a closure that accumulates into a
// captured map or slice. Read-only queries do not use this helper: plain SELECTs take no row
// locks and so have nothing to retry.
func runInTransaction(ctx context.Context, db *gorm.DB, retry *dbretry.Policy, fn func(tx *gorm.DB) error) error {
	return retry.Do(ctx, func() error {
		return db.WithContext(ctx).Transaction(fn)
	})
}

// runInQueryTransaction is runInTransaction for the generated-query API, which owns its own
// transaction helper. Same policy, same re-run contract.
func runInQueryTransaction(ctx context.Context, query *genquery.Query, retry *dbretry.Policy, fn func(query *genquery.Query) error) error {
	return retry.Do(ctx, func() error {
		return query.DBTransaction(fn)
	})
}
