package validate

import (
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// DiscoverByIdentityKeyArgs validates arguments for DiscoverByIdentityKey()
func DiscoverByIdentityKeyArgs(args sdk.DiscoverByIdentityKeyArgs) error {
	if args.Limit != nil {
		if *args.Limit < MinPaginationLimit {
			return fmt.Errorf("limit must be greater than 0")
		}
		if *args.Limit > MaxPaginationLimit {
			return fmt.Errorf("limit exceeds max allowed value of %d", MaxPaginationLimit)
		}
	}

	if args.Offset != nil {
		if *args.Offset > MaxPaginationOffset {
			return fmt.Errorf("offset is too large")
		}
	}

	if args.IdentityKey == nil {
		return fmt.Errorf("identityKey is required")
	}

	hex := primitives.PubKeyHex(args.IdentityKey.ToDERHex())
	if err := hex.Validate(); err != nil {
		return fmt.Errorf("invalid identity key: failed validation check")
	}

	return nil
}
