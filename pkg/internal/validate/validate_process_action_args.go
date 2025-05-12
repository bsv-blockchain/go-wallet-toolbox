package validate

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

func ProcessActionArgs(args *wdk.ProcessActionArgs) error {
	if args.IsNoSend && !args.IsSendWith {
		return fmt.Errorf("inconsistent IsNoSend with IsSendWith - logic error")
	}

	if args.IsNewTx {
		if args.Reference == nil {
			return fmt.Errorf("missing reference argument for new transaction")
		}
		if args.RawTx == nil {
			return fmt.Errorf("missing rawTx argument for new transaction")
		}
		if args.TxID == nil {
			return fmt.Errorf("missing txID argument for new transaction")
		}
	}

	if !args.IsNoSend && args.TxID == nil {
		return fmt.Errorf("missing txID argument for send transaction")
	}

	if args.TxID != nil {
		if err := args.TxID.Validate(); err != nil {
			return fmt.Errorf("invalid txID argument: %w", err)
		}
	}

	return nil
}
