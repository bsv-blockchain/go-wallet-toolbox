package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// OutputBasket represents a user's basket for holding outputs.
type OutputBasket struct {
	Name   string
	UserID int

	CreatedAt time.Time
	UpdatedAt time.Time

	NumberOfDesiredUTXOs    int64
	MinimumDesiredUTXOValue uint64
}

// OutputBasketReadSpecification is used to read OutputBasket entities from the database.
type OutputBasketReadSpecification struct {
	UserID                  *Comparable[int]
	Name                    *Comparable[string]
	NumberOfDesiredUTXOs    *Comparable[int64]
	MinimumDesiredUTXOValue *Comparable[uint64]
}

// OutputBasketUpdateSpecification is used to update OutputBasket entities in the database.
type OutputBasketUpdateSpecification struct {
	UserID                  int
	Name                    *string
	NumberOfDesiredUTXOs    *int64
	MinimumDesiredUTXOValue *uint64
}

// ToWDK converts the OutputBasket entity to its WDK representation.
func (o *OutputBasket) ToWDK() *wdk.TableOutputBasket {
	return &wdk.TableOutputBasket{
		BasketConfiguration: wdk.BasketConfiguration{
			Name:                    primitives.StringUnder300(o.Name),
			NumberOfDesiredUTXOs:    o.NumberOfDesiredUTXOs,
			MinimumDesiredUTXOValue: o.MinimumDesiredUTXOValue,
		},
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
		UserID:    o.UserID,
		IsDeleted: false,
	}
}
