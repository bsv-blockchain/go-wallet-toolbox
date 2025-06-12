package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"gorm.io/gorm"
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
	return &entity.OutputBasket{
		UserID:                  f.user.ID,
		Name:                    "default",
		NumberOfDesiredUTXOs:    int64(number),
		MinimumDesiredUTXOValue: testDesiredUTXOValue,
	}
}
