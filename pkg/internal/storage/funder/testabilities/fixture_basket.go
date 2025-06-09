package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

const (
	desiredUTXONumberToPreferSingleChange = 1
	testDesiredUTXOValue                  = 1000
)

type BasketFixture interface {
	ThatPrefersSingleChange() *wdk.TableOutputBasket
	WithNumberOfDesiredUTXOs(i int) *wdk.TableOutputBasket
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

func (f *basketFixture) ThatPrefersSingleChange() *wdk.TableOutputBasket {
	return f.WithNumberOfDesiredUTXOs(desiredUTXONumberToPreferSingleChange)
}

func (f *basketFixture) WithNumberOfDesiredUTXOs(number int) *wdk.TableOutputBasket {
	return &wdk.TableOutputBasket{
		UserID:   f.user.ID,
		BasketConfiguration: wdk.BasketConfiguration{
			Name:                    "default",
			NumberOfDesiredUTXOs:    int64(number),
			MinimumDesiredUTXOValue: testDesiredUTXOValue,
		},
		CreatedAt: exampleDate,
		UpdatedAt: exampleDate,
		IsDeleted: false,
	}
}
