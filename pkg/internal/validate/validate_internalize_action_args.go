package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func ValidInternalizeActionArgs(args *wdk.InternalizeActionArgs) error {
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

	return nil
}

// WalletInternalizeAction performs wallet-specific validation for internalize actions
func WalletInternalizeAction(keyDeriver *sdk.KeyDeriver, args *wdk.InternalizeActionArgs) error {
	if err := ValidInternalizeActionArgs(args); err != nil {
		return fmt.Errorf("invalid internalize action args: %w", err)
	}

	beef, txIDHash, err := transaction.NewBeefFromAtomicBytes(args.Tx)
	if err != nil {
		return fmt.Errorf("failed to create atomic beef from bytes: %w", err)
	}

	tx := beef.FindAtomicTransactionByHash(txIDHash)
	if tx == nil {
		return fmt.Errorf("atomic beef error: transaction with hash %s not found", txIDHash)
	}

	for _, output := range args.Outputs {
		if err := validateOutput(keyDeriver, *output, tx); err != nil {
			return fmt.Errorf("output validation failed: %w", err)
		}
	}

	return nil
}

// validateOutput validates a single output based on its protocol
func validateOutput(keyDeriver *sdk.KeyDeriver, output wdk.InternalizeOutput, tx *transaction.Transaction) error {
	txOutput := tx.Outputs[output.OutputIndex]

	switch output.Protocol {
	case wdk.WalletPaymentProtocol:
		return validateWalletPaymentOutput(keyDeriver, output, txOutput)
	case wdk.BasketInsertionProtocol:
		return validateBasketInsertionOutput()
	default:
		return fmt.Errorf("unexpected protocol: %s", output.Protocol)
	}
}

// validateWalletPaymentOutput validates a wallet payment output using BRC-29
func validateWalletPaymentOutput(keyDeriver *sdk.KeyDeriver, output wdk.InternalizeOutput, txOutput *transaction.TransactionOutput) error {
	payment := output.PaymentRemittance

	keyID := brc29.KeyID{
		DerivationPrefix: string(payment.DerivationPrefix),
		DerivationSuffix: string(payment.DerivationSuffix),
	}

	expectedLockScript, err := brc29.LockForSelf(brc29.PubHex(payment.SenderIdentityKey), keyID, keyDeriver)
	if err != nil {
		return fmt.Errorf("failed to create expected address: %w", err)
	}

	if txOutput.LockingScript.String() != expectedLockScript.String() {
		return fmt.Errorf("locking script mismatch: expected %s, got %s", expectedLockScript.String(), txOutput.LockingScript.String())
	}

	return nil
}

func validateBasketInsertionOutput() error {
	/*
	   No additional validations...
	*/
	return nil
}
