package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testmode"
)

// TestUserUTXOIndexes_Postgres asserts that AutoMigrate creates the
// composite selection index (user_id, basket_name, reserved_by_id,
// utxo_status, satoshis) on bsv_user_utxos, named idx_user_utxos_selection,
// plus a plain single-column index on reserved_by_id.
//
// Postgres-only: pg_indexes / indexdef column ordering is a Postgres
// catalog concept; SQLite's AutoMigrate accepts the same composite gorm
// tags silently but doesn't expose an equivalent, queryable catalog view.
func TestUserUTXOIndexes_Postgres(t *testing.T) {
	if _, ok := testmode.GetMode().(*testmode.PostgresMode); !ok {
		t.Skip("requires TEST_DB_MODE=postgres (see docs/testing-postgres.md)")
	}

	// given: a freshly migrated database
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	// when: querying the postgres catalog for indexes on bsv_user_utxos
	type indexRow struct {
		IndexName string
		IndexDef  string
	}
	var rows []indexRow
	err := db.DB.Raw(
		`SELECT indexname AS index_name, indexdef AS index_def
		 FROM pg_indexes
		 WHERE schemaname = current_schema() AND tablename = 'bsv_user_utxos'`,
	).Scan(&rows).Error
	require.NoError(t, err)
	require.NotEmpty(t, rows, "expected at least one index on bsv_user_utxos")

	// then: the composite selection index exists with columns in the exact order
	// (user_id, basket_name, reserved_by_id, utxo_status, satoshis)
	var selectionIndexDef string
	var reservedByIDIndexFound bool
	for _, row := range rows {
		if row.IndexName == "idx_user_utxos_selection" {
			selectionIndexDef = row.IndexDef
		}
		if strings.Contains(row.IndexDef, "(reserved_by_id)") {
			reservedByIDIndexFound = true
		}
	}

	require.NotEmpty(t, selectionIndexDef, "expected idx_user_utxos_selection to exist")
	require.Regexp(t,
		`\(user_id, basket_name, reserved_by_id, utxo_status, satoshis\)`,
		selectionIndexDef,
		"idx_user_utxos_selection must cover columns in order: user_id, basket_name, reserved_by_id, utxo_status, satoshis",
	)

	require.True(t, reservedByIDIndexFound, "expected a plain single-column index on reserved_by_id")
}
