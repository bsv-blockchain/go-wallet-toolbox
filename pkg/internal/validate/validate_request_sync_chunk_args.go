package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ValidRequestSyncChunkArgs(args *wdk.RequestSyncChunkArgs) error {
	if args.ToStorageIdentityKey == "" {
		return fmt.Errorf("missing toStorageIdentityKey parameter")
	}

	if args.FromStorageIdentityKey == "" {
		return fmt.Errorf("missing fromStorageIdentityKey parameter")
	}

	if args.IdentityKey == "" {
		return fmt.Errorf("missing user identityKey parameter")
	}

	if args.MaxItems == 0 {
		return fmt.Errorf("maxItems must be greater than 0, got %d", args.MaxItems)
	}

	if args.MaxRoughSize == 0 {
		return fmt.Errorf("maxRoughSize must be greater than 0, got %d", args.MaxRoughSize)
	}

	return nil
}
