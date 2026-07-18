package storage_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// FK-parent seeding helpers.
//
// SQLite (as configured in this codebase) never enables `PRAGMA foreign_keys`,
// so it silently accepts child rows (outputs, user-utxos, commissions, …) that
// reference non-existent parents. Postgres enforces the same foreign keys the
// GORM models declare, so those orphaned rows are rejected. These helpers seed
// the parent rows the entity-CRUD test data assumes, idempotently, so the same
// fixtures work under both engines. They are test-data only — no production code
// or assertions change.
//
// All seeding is written to be engine-agnostic (works on SQLite and Postgres)
// and idempotent (`ON CONFLICT DO NOTHING`), so repeated seeding within one test
// never conflicts.

// ensureParentUser seeds a bsv_users row with an explicit id. identity_key is
// unique per id so it never collides with the real users seeded by the fixture.
func ensureParentUser(t testing.TB, db *gorm.DB, userID int) {
	t.Helper()
	err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.User{
		UserID:        userID,
		IdentityKey:   fmt.Sprintf("seed-parent-identity-%d", userID),
		ActiveStorage: "seed-parent-storage",
	}).Error
	require.NoError(t, err)
}

// ensureParentBasket seeds a bsv_output_baskets row (composite PK user_id+name).
// The parent user is seeded first so the basket→user FK holds on Postgres.
func ensureParentBasket(t testing.TB, db *gorm.DB, userID int, name string) {
	t.Helper()
	ensureParentUser(t, db, userID)
	err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.OutputBasket{
		Name:                    name,
		UserID:                  userID,
		NumberOfDesiredUTXOs:    32,
		MinimumDesiredUTXOValue: 1000,
	}).Error
	require.NoError(t, err)
}

// ensureParentTx seeds a bsv_transactions row with an explicit id (0 allowed).
// Raw SQL is used because GORM treats a zero primary key as "unset" and would
// let the sequence assign one; several fixtures reference transaction_id 0.
// Timestamp columns are left NULL to keep the statement engine-agnostic.
func ensureParentTx(t testing.TB, db *gorm.DB, id uint, userID int) {
	t.Helper()
	// Status is Completed (not Unprocessed) so that seeding a parent transaction
	// never accidentally satisfies a "TxStatus = Unprocessed" output filter.
	err := db.Exec(
		`INSERT INTO bsv_transactions (id, user_id, status, reference, is_outgoing, satoshis, version, lock_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		id, userID, string(wdk.TxStatusCompleted), fmt.Sprintf("seed-parent-tx-ref-%d", id), false, 0, 1, 0,
	).Error
	require.NoError(t, err)
}

// ensureParentTxWith seeds a bsv_transactions row with an explicit id, tx_id and
// status. Used where a test wants an output to reference a transaction with a
// specific id/txid/status: the entity layer's Create ignores the entity's ID
// field (it maps through NewTx), so seeding the row directly is the only way to
// pin a known transaction id that the output's transaction_id can point at.
func ensureParentTxWith(t testing.TB, db *gorm.DB, id uint, userID int, txID string, status wdk.TxStatus) {
	t.Helper()
	err := db.Exec(
		`INSERT INTO bsv_transactions (id, user_id, status, reference, tx_id, is_outgoing, satoshis, version, lock_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		id, userID, string(status), fmt.Sprintf("seed-parent-tx-ref-%d", id), txID, false, 0, 1, 0,
	).Error
	require.NoError(t, err)
}

// ensureParentOutput seeds a bsv_outputs row with an explicit id, satisfying the
// bsv_user_utxos.output_id FK. It seeds the user, basket and a parent transaction
// (id=1) so the output's own composite basket FK and transaction FK hold too.
func ensureParentOutput(t testing.TB, db *gorm.DB, outputID uint, userID int, basketName string) {
	t.Helper()
	ensureParentBasket(t, db, userID, basketName)
	ensureParentTx(t, db, 1, userID)
	name := basketName
	err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.Output{
		Model:         gorm.Model{ID: outputID},
		UserID:        userID,
		TransactionID: 1,
		BasketName:    &name,
	}).Error
	require.NoError(t, err)
}

// seedUTXOParents seeds every FK parent a bare UserUTXO row needs: the user, the
// referenced basket, and the referenced output (which itself pulls in a parent
// transaction). It is idempotent, so many UTXOs sharing a user/basket are fine.
func seedUTXOParents(t testing.TB, db *gorm.DB, userID int, basketName string, outputID uint) {
	t.Helper()
	ensureParentOutput(t, db, outputID, userID, basketName)
}

// createUTXO seeds the FK parents a UserUTXO row references (user, basket, output,
// and — when set — the reserving transaction) and then creates the UTXO through
// the production entity path. Drop-in for provider.UserUTXOEntity().Create so the
// entity-CRUD tests stay unchanged apart from the parent seeding.
func createUTXO(t testing.TB, provider *storage.Provider, utxo *entity.UserUTXO) error {
	t.Helper()
	db := provider.Database.DB
	seedUTXOParents(t, db, utxo.UserID, utxo.BasketName, utxo.OutputID)
	if utxo.ReservedByID != nil {
		ensureParentTx(t, db, *utxo.ReservedByID, utxo.UserID)
	}
	return provider.UserUTXOEntity().Create(t.Context(), utxo)
}
