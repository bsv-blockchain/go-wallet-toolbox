package wdk

import (
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

// BasketConfiguration is a struct that defines the configuration of the output basket
type BasketConfiguration struct {
	Name                    primitives.StringUnder300 `json:"name"`
	NumberOfDesiredUTXOs    int64                     `json:"numberOfDesiredUTXOs"`
	MinimumDesiredUTXOValue uint64                    `json:"minimumDesiredUTXOValue"`
}

// TableOutputBasket is a struct that holds the output baskets details
type TableOutputBasket struct {
	BasketConfiguration `json:",inline"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	//BasketID            int       `json:"basketId"`
	UserID    int  `json:"userId"`
	IsDeleted bool `json:"isDeleted"`
}

// DefaultBasketConfiguration returns a default basket configuration
func DefaultBasketConfiguration() BasketConfiguration {
	return BasketConfiguration{
		Name:                    BasketNameForChange,
		NumberOfDesiredUTXOs:    NumberOfDesiredUTXOsForChange,
		MinimumDesiredUTXOValue: MinimumDesiredUTXOValueForChange,
	}
}
