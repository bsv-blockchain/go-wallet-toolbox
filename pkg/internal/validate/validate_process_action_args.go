package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ProcessActionArgs(args *wdk.ProcessActionArgs) error {
	if args.IsSendWith && len(args.SendWith) == 0 {
		return fmt.Errorf("IsSendWith is true but no sendWith arguments provided")
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

	if args.TxID != nil {
		if err := args.TxID.Validate(); err != nil {
			return fmt.Errorf("invalid txID argument: %w", err)
		}
	}

	return nil
}
