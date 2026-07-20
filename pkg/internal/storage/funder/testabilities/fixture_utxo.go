package testabilities

import (
	"fmt"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

var FirstCreatedAt = time.Date(2006, 0o2, 0o1, 15, 4, 5, 7, time.UTC)

type UserUTXOFixture interface {
	OwnedBy(user testusers.User) UserUTXOFixture
	InBasket(basket *entity.OutputBasket) UserUTXOFixture
	P2PKH() UserUTXOFixture
	WithSatoshis(sats int64) UserUTXOFixture
	WithStatus(status wdk.UTXOStatus) UserUTXOFixture
	Stored()
}

type UTXODatabase interface {
	Save(utxo *models.Output)
}

var defaultBasket = entity.OutputBasket{
	Name:                    wdk.BasketNameForChange,
	NumberOfDesiredUTXOs:    30,
	MinimumDesiredUTXOValue: 1000,
	UserID:                  1,
}

type userUtxoFixture struct {
	parent             UTXODatabase
	t                  testing.TB
	index              uint
	userID             int
	vout               uint32
	satoshis           uint64
	estimatedInputSize uint64
	basket             *entity.OutputBasket
	status             wdk.UTXOStatus
}

func newUtxoFixture(t testing.TB, parent UTXODatabase, index uint) *userUtxoFixture {
	basket := defaultBasket
	return &userUtxoFixture{
		t:                  t,
		parent:             parent,
		index:              index,
		basket:             &basket,
		userID:             1,
		vout:               uint32(index), //nolint:gosec // test fixture, index is always small
		satoshis:           1,
		estimatedInputSize: txutils.P2PKHEstimatedInputSize,
		status:             wdk.UTXOStatusUnproven,
	}
}

func (f *userUtxoFixture) InBasket(basket *entity.OutputBasket) UserUTXOFixture {
	f.basket = basket
	return f
}

func (f *userUtxoFixture) OwnedBy(user testusers.User) UserUTXOFixture {
	f.userID = user.ID
	f.basket.UserID = user.ID
	return f
}

func (f *userUtxoFixture) P2PKH() UserUTXOFixture {
	f.estimatedInputSize = txutils.P2PKHEstimatedInputSize
	return f
}

func (f *userUtxoFixture) WithSatoshis(satoshis int64) UserUTXOFixture {
	if satoshis < 0 {
		f.t.Fatalf("satoshis must be a positive number, got %d", satoshis)
	}
	f.satoshis = uint64(satoshis) //nolint:gosec // test fixture, satoshis is always positive
	return f
}

func (f *userUtxoFixture) WithStatus(status wdk.UTXOStatus) UserUTXOFixture {
	f.status = status
	return f
}

func (f *userUtxoFixture) Stored() {
	if f.satoshis == 0 {
		return
	}

	basketId := uint(1) // dummy basket id

	txStatus := wdk.TxStatusUnproven
	if f.status == wdk.UTXOStatusMined {
		txStatus = wdk.TxStatusCompleted
	} else if f.status == wdk.UTXOStatusSending {
		txStatus = wdk.TxStatusSending
	}

	utxo := &models.Output{
		UserID:             f.userID,
		OutputID:           f.index,
		Satoshis:           int64(f.satoshis),
		BasketID:           &basketId,
		Spendable:          true,
		Basket: &models.OutputBasket{
			Name:                    f.basket.Name,
			UserID:                  f.basket.UserID,
			NumberOfDesiredUTXOs:    f.basket.NumberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: f.basket.MinimumDesiredUTXOValue,
		},
		Transaction: &models.Transaction{
			UserID:    f.userID,
			Status:    txStatus,
			Reference: fmt.Sprintf("test-funder-utxo-%d", f.index),
		},
	}

	f.parent.Save(utxo)
}
