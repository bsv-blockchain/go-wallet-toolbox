package testabilities

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
)

const (
	desiredUTXONumberToPreferSingleChange = 1
	testDesiredUTXOValue                  = 1000
)

type BasketFixture interface {
	ThatPrefersSingleChange() *entity.OutputBasket
	WithNumberOfDesiredUTXOs(i int) *entity.OutputBasket
}

type basketFixture struct {
	testing.TB

	db   *gorm.DB
	user testusers.User
}

func newBasketFixture(t testing.TB, db *gorm.DB, user testusers.User) *basketFixture {
	return &basketFixture{
		TB:   t,
		db:   db,
		user: user,
	}
}

func (f *basketFixture) ThatPrefersSingleChange() *entity.OutputBasket {
	return f.WithNumberOfDesiredUTXOs(desiredUTXONumberToPreferSingleChange)
}

func (f *basketFixture) WithNumberOfDesiredUTXOs(number int) *entity.OutputBasket {
	// Persist the basket for real (rather than assuming a hardcoded dummy ID) so its BasketID
	// matches whatever bsv_outputs.basketId rows actually reference, and so it's distinguishable
	// from other named baskets created later in the same test.
	basket := &models.OutputBasket{
		Name:                    "default",
		UserID:                  f.user.ID,
		NumberOfDesiredUTXOs:    int64(number),
		MinimumDesiredUTXOValue: testDesiredUTXOValue,
	}
	err := f.db.Where("name = ? AND userId = ?", basket.Name, basket.UserID).FirstOrCreate(basket).Error
	require.NoError(f.TB, err)

	// Return the caller-requested values (not the just-created row's), since GORM silently
	// substitutes the column's `default:` tag value whenever a default-tagged field is the Go
	// zero value (e.g. NumberOfDesiredUTXOs: 0) on Create, which would otherwise mask exactly the
	// "zero" and "negative" desired-UTXO edge cases these fixtures exist to test.
	return &entity.OutputBasket{
		ID:                      basket.BasketID,
		UserID:                  f.user.ID,
		Name:                    "default",
		NumberOfDesiredUTXOs:    int64(number),
		MinimumDesiredUTXOValue: testDesiredUTXOValue,
	}
}
