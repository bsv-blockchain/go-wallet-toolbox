package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
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
	UserID              int       `json:"userId"`
	IsDeleted           bool      `json:"isDeleted"`

	// BasketNumID is to keep interoperability via API, NOTE that this field is not a primary key in the database
	BasketID int `json:"basketId"`
}

// DefaultBasketConfiguration returns a default basket configuration
func DefaultBasketConfiguration() BasketConfiguration {
	return BasketConfiguration{
		Name:                    BasketNameForChange,
		NumberOfDesiredUTXOs:    NumberOfDesiredUTXOsForChange,
		MinimumDesiredUTXOValue: MinimumDesiredUTXOValueForChange,
	}
}

// TableOutputBaskets is a slice of TableOutputBasket
type TableOutputBaskets = []TableOutputBasket

// FindOutputBasketsArgs holds the arguments for finding output baskets
type FindOutputBasketsArgs struct {
	UserID                  *int    `json:"userId,omitempty"`
	Name                    *string `json:"name,omitempty"`
	NumberOfDesiredUTXOs    *int64  `json:"numberOfDesiredUTXOs,omitempty"`
	MinimumDesiredUTXOValue *uint64 `json:"minimumDesiredUTXOValue,omitempty"`
}
