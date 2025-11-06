package repo

import (
	"context"

	"gorm.io/gorm"
)

type TransactionFunc = func(ctx context.Context) error

func newDBTransaction(ctx context.Context, db *gorm.DB, action TransactionFunc) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ctx = ctxWithDBTx(ctx, tx)
		return action(ctx)
	})
}

type dbTxKey struct{}

func ctxWithDBTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, dbTxKey{}, tx)
}

func dbTxFromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(dbTxKey{}).(*gorm.DB)
	return tx, ok
}
