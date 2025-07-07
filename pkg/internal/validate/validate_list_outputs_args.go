package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const (
	MaxPaginationLimit  = 10000
	MaxPaginationOffset = 1_000_000
	MinPaginationLimit  = 1
)

func ListOutputsArgs(args *wdk.ListOutputsArgs) error {
	if args == nil {
		return fmt.Errorf("args cannot be nil")
	}

	if err := args.TagQueryMode.Validate(); err != nil {
		return fmt.Errorf("invalid tagQueryMode: %s", *args.TagQueryMode)
	}

	if args.Limit < MinPaginationLimit {
		return fmt.Errorf("limit must be greater than 0")
	}
	if args.Limit > MaxPaginationLimit {
		return fmt.Errorf("limit exceeds max allowed value of %d", MaxPaginationLimit)
	}
	if args.Offset > MaxPaginationOffset {
		return fmt.Errorf("offset is too large")
	}

	for _, txid := range args.KnownTxids {
		if err := primitives.TXIDHexString(txid).Validate(); err != nil {
			return fmt.Errorf("invalid txid: %w", err)
		}
	}

	return nil
}
