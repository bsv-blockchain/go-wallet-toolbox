package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ValidBasketConfiguration(config *wdk.BasketConfiguration) error {
	if err := config.Name.Validate(); err != nil {
		return fmt.Errorf("invalid Basket name: %w", err)
	}

	// MinimumDesiredUTXOValue is only meaningful for the change basket: funder.Fund divides
	// by it when deciding how many change outputs to create, so 0 there causes a divide-by-zero
	// panic. For non-change baskets the field is not applicable (see wdk.NonChangeBasketConfiguration)
	// and 0 is a legitimate value, so the check is scoped to the change basket name only.
	if string(config.Name) == wdk.BasketNameForChange && config.MinimumDesiredUTXOValue == 0 {
		return fmt.Errorf("minimumDesiredUTXOValue must be greater than 0 for the change basket, got %d", config.MinimumDesiredUTXOValue)
	}

	return nil
}
