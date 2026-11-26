package genquery

// This file is written by hand: the generator only writes gen.go and the *.gen.go files.

import (
	"database/sql"

	"gorm.io/gorm"
)

// BindTx returns a copy of q whose tables run on tx's connection: the copy DBTransaction hands
// to its callback. It keeps the models and parsed schemas q was built with, which makes it much
// cheaper than Use(tx) for a short-lived handle, such as one per database transaction.
//
// Like the copy DBTransaction makes, the tables keep q's statement context, so queries take
// their context from WithContext.
func (q *Query) BindTx(tx *gorm.DB) *Query {
	return q.clone(tx)
}

// RunInTransaction is DBTransaction without the copy when there is nothing to rebind. When q
// is already bound to a database transaction, the transaction fc runs in is a savepoint on that
// same connection, and fc gets q itself: a copy bound to tx would run every statement on the
// very connection q already uses. Otherwise fc gets the copy DBTransaction hands over.
func (q *Query) RunInTransaction(fc func(tx *Query) error, opts ...*sql.TxOptions) error {
	return q.db.Transaction(func(tx *gorm.DB) error {
		if onSameSQLTx(q.db, tx) {
			return fc(q)
		}
		return fc(q.clone(tx))
	}, opts...)
}

// onSameSQLTx reports whether a and b both run on the same *sql.Tx. Any other connection pool,
// such as gorm's prepared-statement wrappers, reports false.
func onSameSQLTx(a, b *gorm.DB) bool {
	aTx, ok := a.Statement.ConnPool.(*sql.Tx)
	if !ok {
		return false
	}
	bTx, ok := b.Statement.ConnPool.(*sql.Tx)
	return ok && aTx == bTx
}
