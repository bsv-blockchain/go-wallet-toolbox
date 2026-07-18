package syncrepo_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedSpenderTx seeds a bsv_transactions row with an explicit id so an output's
// spent_by FK (bsv_outputs.spent_by → bsv_transactions.id) holds on Postgres.
//
// The BRC-40 conformance vectors and guard tests reference spent-by transaction
// ids (42, 99, 7, 43, …) that the fixtures never create. SQLite never enables
// PRAGMA foreign_keys and silently accepted the orphaned spent_by; Postgres
// enforces the FK. Seeding a matching transaction keeps the exact spentBy value
// the tests assert on. Idempotent; id 0 is a no-op (means "not spent").
func seedSpenderTx(t *testing.T, db *gorm.DB, id *uint) {
	t.Helper()
	if id == nil || *id == 0 {
		return
	}
	err := db.Exec(
		`INSERT INTO bsv_transactions (id, user_id, reference) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
		*id, 1, fmt.Sprintf("syncrepo-seed-spender-tx-%d", *id),
	).Error
	require.NoError(t, err)
}
