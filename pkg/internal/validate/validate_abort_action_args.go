package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ValidAbortActionArgs(args *wdk.AbortActionArgs) error {
	if args == nil {
		return fmt.Errorf("args cannot be nil")
	}

	if args.Reference == "" {
		return fmt.Errorf("missing reference argument for abort action")
	}

	if err := args.Reference.Validate(); err != nil {
		return fmt.Errorf("invalid reference format: %w", err)
	}

	return nil
}
