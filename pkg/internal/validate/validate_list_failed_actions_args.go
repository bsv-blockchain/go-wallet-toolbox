package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ListFailedActionsArgs(args *wdk.ListFailedActionsArgs) error {
	if args == nil {
		return fmt.Errorf("args cannot be nil")
	}

	if args.Limit > MaxPaginationLimit {
		return fmt.Errorf("limit must be less than or equal to %d", MaxPaginationLimit)
	}
	if args.Offset > MaxPaginationOffset {
		return fmt.Errorf("offset must be less than or equal to %d", MaxPaginationOffset)
	}

	if !args.SeekPermission.Value() {
		return fmt.Errorf("operation not allowed without permission (seekPermission=false)")
	}

	if !args.IncludeInputs.Value() {
		if args.IncludeInputUnlockingScripts.Value() {
			return fmt.Errorf("includeInputUnlockingScripts cannot be true when includeInputs is false")
		}
		if args.IncludeInputSourceLockingScripts.Value() {
			return fmt.Errorf("includeInputSourceLockingScripts cannot be true when includeInputs is false")
		}
	}

	if !args.IncludeOutputs.Value() && args.IncludeOutputLockingScripts.Value() {
		return fmt.Errorf("includeOutputLockingScripts cannot be true when includeOutputs is false")
	}

	return nil
}
