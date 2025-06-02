package validate

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

func ListActionsArgs(args *wdk.ListActionsArgs) error {
	if args == nil {
		return fmt.Errorf("args cannot be nil")
	}

	if args.LabelQueryMode != nil {
		if err := args.LabelQueryMode.Validate(); err != nil {
			return fmt.Errorf("invalid labelQueryMode: %s", *args.LabelQueryMode)
		}
	}

	if args.Limit > MaxPaginationLimit {
		return fmt.Errorf("limit must be less than or equal to %d", MaxPaginationLimit)
	}
	if args.Offset > MaxPaginationOffset {
		return fmt.Errorf("offset must be less than or equal to %d", MaxPaginationOffset)
	}

	for _, label := range args.Labels {
		if err := validateLabel(label); err != nil {
			return fmt.Errorf("invalid label: %w", err)
		}
	}

	if !args.SeekPermissions.Value() {
		return fmt.Errorf("operation not allowed without permission (seekPermissions=false)")
	}

	return nil
}

func validateLabel(label primitives.StringUnder300) error {
	if len(label) == 0 || len(label) > 300 {
		return fmt.Errorf("label must be 1-300 characters")
	}
	return nil
}
