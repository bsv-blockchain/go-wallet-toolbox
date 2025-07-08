package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ValidBasketConfiguration(config *wdk.BasketConfiguration) error {
	if err := config.Name.Validate(); err != nil {
		return fmt.Errorf("invalid Basket name: %w", err)
	}
	return nil
}
