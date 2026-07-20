package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/brc114"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func ListActionsArgs(args *wdk.ListActionsArgs) error {
	if args == nil {
		return fmt.Errorf("args cannot be nil")
	}

	if err := args.LabelQueryMode.Validate(); err != nil {
		return fmt.Errorf("invalid labelQueryMode: %s", *args.LabelQueryMode)
	}

	if args.Limit > MaxPaginationLimit {
		return fmt.Errorf("limit must be less than or equal to %d", MaxPaginationLimit)
	}
	if args.Offset > MaxPaginationOffset {
		return fmt.Errorf("offset must be less than or equal to %d", MaxPaginationOffset)
	}

	labelNames := make([]string, len(args.Labels))
	for i, label := range args.Labels {
		if err := validateLabel(label); err != nil {
			return fmt.Errorf("invalid label: %w", err)
		}
		labelNames[i] = string(label)
	}

	// BRC-114 time control labels are validated (and later stripped) for TS parity.
	if _, err := brc114.ParseActionTimeLabels(labelNames); err != nil {
		return err
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

func validateLabel(label primitives.StringUnder300) error {
	if len(label) == 0 || len(label) > 300 {
		return fmt.Errorf("label must be 1-300 characters")
	}
	return nil
}
