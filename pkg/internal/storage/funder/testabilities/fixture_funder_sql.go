package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/stretchr/testify/require"
)

type FunderFixture interface {
	NewFunderService() *funder.SQL
	UTXO() UserUTXOFixture
	BasketFor(user testusers.User) BasketFixture
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
	repo := f.db.CreateRepositories().UTXOs
	return funder.NewSQL(logging.NewTestLogger(f.t), repo, feeModel)
}

func (f *funderFixture) UTXO() UserUTXOFixture {
	index := uint(len(f.createdUTXOs))
	return newUtxoFixture(f.t, f, index)
}

func (f *funderFixture) Save(utxo *models.UserUTXO) {
	err := f.db.DB.Create(&utxo).Error
	require.NoError(f.t, err)
	f.createdUTXOs = append(f.createdUTXOs, utxo)
}

func (f *funderFixture) BasketFor(user testusers.User) BasketFixture {
	return newBasketFixture(f.t, f.db.DB, user)
}
