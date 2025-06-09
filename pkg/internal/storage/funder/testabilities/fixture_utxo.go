package testabilities

import (
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

var FirstCreatedAt = time.Date(2006, 02, 01, 15, 4, 5, 7, time.UTC)

type UserUTXOFixture interface {
	OwnedBy(user testusers.User) UserUTXOFixture
	InBasket(basket *wdk.TableOutputBasket) UserUTXOFixture
	P2PKH() UserUTXOFixture
	WithSatoshis(sats int64) UserUTXOFixture
	Stored()
}

type UTXODatabase interface {
	Save(utxo *models.UserUTXO)
}

var defaultBasket = wdk.TableOutputBasket{
	CreatedAt: FirstCreatedAt,
	UpdatedAt: FirstCreatedAt,
	IsDeleted: false,
	BasketConfiguration: wdk.BasketConfiguration{
		Name:                    wdk.BasketNameForChange,
		NumberOfDesiredUTXOs:    30,
		MinimumDesiredUTXOValue: 1000,
	},
	UserID: 1,
}

type userUtxoFixture struct {
	parent             UTXODatabase
	t                  testing.TB
	index              uint
	userID             int
	vout               uint32
	satoshis           uint64
	estimatedInputSize uint64
	basket             *wdk.TableOutputBasket
}

func newUtxoFixture(t testing.TB, parent UTXODatabase, index uint) *userUtxoFixture {
	basket := defaultBasket
	return &userUtxoFixture{
		t:                  t,
		parent:             parent,
		index:              index,
		basket:             &basket,
		userID:             1,
		vout:               uint32(index),
		satoshis:           1,
		estimatedInputSize: txutils.P2PKHEstimatedInputSize,
	}
}

func (f *userUtxoFixture) InBasket(basket *wdk.TableOutputBasket) UserUTXOFixture {
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
	f.satoshis = uint64(satoshis)
	return f
}

func (f *userUtxoFixture) Stored() {
	if f.satoshis == 0 {
		return
	}

	utxo := &models.UserUTXO{
		UserID:             f.userID,
		OutputID:           f.index,
		Satoshis:           f.satoshis,
		EstimatedInputSize: f.estimatedInputSize,
		CreatedAt:          FirstCreatedAt.Add(time.Duration(f.index) * time.Second),
		BasketName:         string(f.basket.Name),
		Basket: &models.OutputBasket{
			CreatedAt:               FirstCreatedAt,
			UpdatedAt:               FirstCreatedAt,
			DeletedAt:               gorm.DeletedAt{},
			Name:                    string(f.basket.Name),
			UserID:                  f.basket.UserID,
			NumberOfDesiredUTXOs:    f.basket.NumberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: f.basket.MinimumDesiredUTXOValue,
		},
	}

	f.parent.Save(utxo)
}
