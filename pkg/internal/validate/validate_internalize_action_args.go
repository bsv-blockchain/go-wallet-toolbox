package validate

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ValidInternalizeActionArgs(args *wdk.InternalizeActionArgs) error {
	return ValidInternalizeActionArgsWithServices(context.Background(), args, nil)
}

// ValidInternalizeActionArgsWithServices validates internalize action arguments including BEEF validation when services are provided
func ValidInternalizeActionArgsWithServices(ctx context.Context, args *wdk.InternalizeActionArgs, services wdk.Services) error {
	if len(args.Tx) == 0 {
		return fmt.Errorf("tx cannot be empty")
	}
	if len(args.Outputs) == 0 {
		return fmt.Errorf("outputs cannot be empty")
	}
	if err := args.Description.Validate(); err != nil {
		return fmt.Errorf("description must be %w", err)
	}
	for i, output := range args.Outputs {
		if err := output.Validate(); err != nil {
			return fmt.Errorf("invalid output [%d]: %w", i, err)
		}
	}
	for i, label := range args.Labels {
		if err := label.Validate(); err != nil {
			return fmt.Errorf("label [%d] must be %w", i, err)
		}
	}

	if services != nil {
		if err := validateBeef(ctx, args.Tx, services); err != nil {
			return fmt.Errorf("beef validation failed: %w", err)
		}
	}

	return nil
}

// validateBeef validates the BEEF transaction using the provided services
func validateBeef(ctx context.Context, txBytes []byte, services wdk.Services) error {
	beef, txIDHash, err := transaction.NewBeefFromAtomicBytes(txBytes)
	if err != nil {
		return fmt.Errorf("failed to create atomic beef from bytes: %w", err)
	}

	ok, err := beef.Verify(ctx, services, false)
	if err != nil {
		return fmt.Errorf("beef verification failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("beef is invalid")
	}

	tx := beef.FindAtomicTransactionByHash(txIDHash)
	if tx == nil {
		return fmt.Errorf("atomic beef error: transaction with hash %s not found", txIDHash)
	}

	return nil
}
