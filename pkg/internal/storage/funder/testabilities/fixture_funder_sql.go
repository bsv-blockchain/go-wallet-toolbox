package testabilities

import (
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
	createdUTXOs []*models.Output
}

func newFixture(t testing.TB, db *database.Database) FunderFixture {
	return &funderFixture{
		t:            t,
		db:           db,
		createdUTXOs: make([]*models.Output, 0),
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

func (f *funderFixture) Save(utxo *models.Output) {
	// Create the owning transaction and basket explicitly first and backfill the FKs ourselves:
	// GORM's belongsTo auto-backfill isn't reliable here once OutputID is set explicitly (as it
	// is for every UTXO fixture, to keep IDs deterministic across a test's multiple Stored()
	// calls), and different basket names must resolve to different real basket IDs rather than
	// all sharing one hardcoded dummy ID.
	if utxo.Transaction != nil {
		err := f.db.DB.Create(utxo.Transaction).Error
		require.NoError(f.t, err)
		utxo.TransactionID = utxo.Transaction.TransactionID
		utxo.Transaction = nil
	}

	if utxo.Basket != nil {
		basket := utxo.Basket
		err := f.db.DB.Where("name = ? AND userId = ?", basket.Name, basket.UserID).FirstOrCreate(basket).Error
		require.NoError(f.t, err)
		utxo.BasketID = &basket.BasketID
		utxo.Basket = nil
	}

	err := f.db.DB.Create(utxo).Error
	require.NoError(f.t, err)
	f.createdUTXOs = append(f.createdUTXOs, utxo)
}

func (f *funderFixture) BasketFor(user testusers.User) BasketFixture {
	return newBasketFixture(f.t, f.db.DB, user)
}

func (f *funderFixture) GormDB() *gorm.DB {
	return f.db.DB
}
