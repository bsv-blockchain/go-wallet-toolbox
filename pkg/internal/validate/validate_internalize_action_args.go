package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ValidInternalizeActionArgs(args *wdk.InternalizeActionArgs) error {
	if len(args.Tx) == 0 {
		return fmt.Errorf("tx cannot be empty")
	}
	if len(args.Outputs) == 0 {
		return fmt.Errorf("outputs cannot be empty")
	}
	if err := args.Description.Validate(); err != nil {
		return fmt.Errorf("description must be %w", err)
	}
	for i, output := range args.Outputs {
		if err := output.Validate(); err != nil {
			return fmt.Errorf("invalid output [%d]: %w", i, err)
		}
	}
	for i, label := range args.Labels {
		if err := label.Validate(); err != nil {
			return fmt.Errorf("label [%d] must be %w", i, err)
		}
	}

	return nil
}
