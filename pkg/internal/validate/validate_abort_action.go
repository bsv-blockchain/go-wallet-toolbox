package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ValidAbortActionArgs(args *wdk.AbortActionArgs) error {
	if args == nil {
		return fmt.Errorf("args cannot be nil")
	}

	if args.Reference == nil {
		return fmt.Errorf("missing reference argument for new transaction")
	}

	return nil
}
