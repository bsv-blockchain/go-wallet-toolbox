package testabilities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// AssertStorageInvariants checks cross-table money-safety invariants that must
// hold after ANY storage scenario. Extend as waves land (Decision Record v1, W0-3).
//
// Portability: predicates run on both SQLite and Postgres. Boolean comparisons
// bind a Go bool as a query parameter (rendered 0/1 on SQLite, true/false on
// Postgres by the GORM dialector) instead of embedding a literal. Table names are
// the migrated names (note KnownTx pluralises to bsv_known_txes, not _txs).
func AssertStorageInvariants(t testing.TB, db *gorm.DB) {
	t.Helper()

	var n int64

	// 1. An output is never simultaneously spendable and spent.
	assert.NoError(t, db.Raw(
		`SELECT count(*) FROM bsv_outputs WHERE spendable = ? AND spent_by IS NOT NULL`,
		true,
	).Scan(&n).Error)
	assert.Zerof(t, n, "%d outputs are spendable AND spent_by-claimed", n)

	// 2. No completed user transaction without a merkle proof on its KnownTx.
	assert.NoError(t, db.Raw(`
		SELECT count(*) FROM bsv_transactions tx
		JOIN bsv_known_txes ktx ON ktx.tx_id = tx.tx_id
		WHERE tx.status = ? AND ktx.merkle_path IS NULL`,
		string(wdk.TxStatusCompleted),
	).Scan(&n).Error)
	assert.Zerof(t, n, "%d completed transactions lack a merkle proof", n)

	// 3. A funder-spendable coin — an UNRESERVED UserUTXO row — must project a live
	//    output. Reserved rows (reserved_by_id set) are legitimately mid-flight:
	//    CreateAction marks their output not-spendable inside the funding txn before
	//    the transaction is processed or aborted, so they are excluded here.
	assert.NoError(t, db.Raw(`
		SELECT count(*) FROM bsv_user_utxos uu
		JOIN bsv_outputs o ON o.id = uu.output_id
		WHERE uu.reserved_by_id IS NULL AND (o.spendable = ? OR o.spent_by IS NOT NULL)`,
		false,
	).Scan(&n).Error)
	assert.Zerof(t, n, "%d unreserved user_utxos rows point at dead outputs", n)
}
