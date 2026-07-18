package testabilities

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

type FunderFixture interface {
	NewFunderService() *funder.SQL
	NewFunderServiceWithFeeRate(satPerKb int64) *funder.SQL
	UTXO() UserUTXOFixture
	BasketFor(user testusers.User) BasketFixture
	GormDB() *gorm.DB
}

var feeModel = defs.FeeModel{
	Type:  defs.SatPerKB,
	Value: 1,
}

type funderFixture struct {
	t            testing.TB
	db           *database.Database
	createdUTXOs []*models.UserUTXO
}

func newFixture(t testing.TB, db *database.Database) FunderFixture {
	return &funderFixture{
		t:            t,
		db:           db,
		createdUTXOs: make([]*models.UserUTXO, 0),
	}
}

func (f *funderFixture) NewFunderService() *funder.SQL {
	return f.NewFunderServiceWithFeeRate(feeModel.Value)
}

func (f *funderFixture) NewFunderServiceWithFeeRate(satPerKb int64) *funder.SQL {
	repo := f.db.CreateRepositories().UTXOs
	model := defs.FeeModel{Type: defs.SatPerKB, Value: satPerKb}
	return funder.NewSQL(logging.NewTestLogger(f.t), repo, model, defs.DefaultChangeBasket().MaxChangeOutputsPerTx)
}

func (f *funderFixture) UTXO() UserUTXOFixture {
	index := uint(len(f.createdUTXOs))
	return newUtxoFixture(f.t, f, index)
}

func (f *funderFixture) Save(utxo *models.UserUTXO) {
	// Seed the FK parents this UserUTXO references so the row is accepted on
	// FK-enforcing engines (Postgres). SQLite never enables PRAGMA foreign_keys,
	// so it silently accepted these orphaned rows. Test-data only; idempotent.
	f.seedUTXOParents(utxo)

	err := f.db.DB.Create(&utxo).Error
	require.NoError(f.t, err)
	f.createdUTXOs = append(f.createdUTXOs, utxo)
}

// seedUTXOParents idempotently seeds the user, basket, a parent transaction and
// the parent output referenced by the given UserUTXO (bsv_user_utxos has FKs on
// output_id and the composite (user_id, basket_name); the basket in turn has a
// user FK, and the output a transaction FK). Written engine-agnostically (no
// engine-specific functions; explicit ids so output_id=0 is representable).
func (f *funderFixture) seedUTXOParents(utxo *models.UserUTXO) {
	db := f.db.DB

	require.NoError(f.t, db.Exec(
		`INSERT INTO bsv_users (user_id, identity_key, active_storage) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
		utxo.UserID, fmt.Sprintf("funder-seed-identity-%d", utxo.UserID), "funder-seed-storage",
	).Error)

	require.NoError(f.t, db.Exec(
		`INSERT INTO bsv_output_baskets (user_id, name) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		utxo.UserID, utxo.BasketName,
	).Error)

	require.NoError(f.t, db.Exec(
		`INSERT INTO bsv_transactions (id, user_id, reference) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
		1, utxo.UserID, "funder-seed-tx-ref",
	).Error)

	require.NoError(f.t, db.Exec(
		`INSERT INTO bsv_outputs (id, user_id, transaction_id, basket_name, vout, satoshis, spendable, change)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		utxo.OutputID, utxo.UserID, 1, utxo.BasketName, utxo.OutputID, 0, false, false,
	).Error)
}

func (f *funderFixture) BasketFor(user testusers.User) BasketFixture {
	return newBasketFixture(f.t, f.db.DB, user)
}

func (f *funderFixture) GormDB() *gorm.DB {
	return f.db.DB
}
