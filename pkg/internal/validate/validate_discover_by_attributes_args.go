package validate

import (
	"errors"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// DiscoverByAttributesArgs validates arguments for DiscoverByAttributes()
func DiscoverByAttributesArgs(args sdk.DiscoverByAttributesArgs) error {
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

	if len(args.Attributes) == 0 {
		return errors.New("attributes must be provided")
	}

	for key := range args.Attributes {
		keyLen := len(key)
		if keyLen < 1 || keyLen > 50 {
			return fmt.Errorf("attributes field name %s must be between %d and %d but has %d length", key, 1, 50, keyLen)
		}
	}

	return nil
}
