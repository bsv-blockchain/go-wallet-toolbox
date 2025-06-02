package validate

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

func ValidRelinquishOutputArgs(args *sdk.RelinquishOutputArgs) error {
	err := primitives.OutpointString(args.Output).Validate()
	if err != nil {
		return fmt.Errorf("invalid outpoint: %w", err)
	}

	return nil
}
