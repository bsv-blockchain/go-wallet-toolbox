package validate

import (
	"context"
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

// WalletInternalizeAction performs wallet-specific validation for internalize actions.
func WalletInternalizeAction(keyDeriver *sdk.KeyDeriver, args *wdk.InternalizeActionArgs) error {
	return internalizeOutputs(args, func(output wdk.InternalizeOutput, txOutput *transaction.TransactionOutput) error {
		return validateOutput(keyDeriver, output, txOutput)
	})
}

// WalletInternalizeActionDerived validates wallet-payment outputs with a
// deriver that does not return a private key.
func WalletInternalizeActionDerived(ctx context.Context, deriver brc29.PublicDeriver, args *wdk.InternalizeActionArgs) error {
	return internalizeOutputs(args, func(output wdk.InternalizeOutput, txOutput *transaction.TransactionOutput) error {
		return validateOutputDerived(ctx, deriver, output, txOutput)
	})
}

func internalizeOutputs(args *wdk.InternalizeActionArgs, check func(wdk.InternalizeOutput, *transaction.TransactionOutput) error) error {
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
		if err := check(*output, tx.Outputs[output.OutputIndex]); err != nil {
			return fmt.Errorf("output validation failed: %w", err)
		}
	}

	return nil
}

// validateOutput validates a single output based on its protocol.
func validateOutput(keyDeriver *sdk.KeyDeriver, output wdk.InternalizeOutput, txOutput *transaction.TransactionOutput) error {
	switch output.Protocol {
	case wdk.WalletPaymentProtocol:
		return validateWalletPaymentOutput(keyDeriver, output, txOutput)
	case wdk.BasketInsertionProtocol:
		return validateBasketInsertionOutput()
	default:
		return fmt.Errorf("unexpected protocol: %s", output.Protocol)
	}
}

func validateOutputDerived(ctx context.Context, deriver brc29.PublicDeriver, output wdk.InternalizeOutput, txOutput *transaction.TransactionOutput) error {
	switch output.Protocol {
	case wdk.WalletPaymentProtocol:
		return validateWalletPaymentOutputDerived(ctx, deriver, output, txOutput)
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

func validateWalletPaymentOutputDerived(ctx context.Context, deriver brc29.PublicDeriver, output wdk.InternalizeOutput, txOutput *transaction.TransactionOutput) error {
	payment := output.PaymentRemittance
	keyID := brc29.KeyID{
		DerivationPrefix: string(payment.DerivationPrefix),
		DerivationSuffix: string(payment.DerivationSuffix),
	}
	sender, err := brc29.PubHex(payment.SenderIdentityKey).PublicKey()
	if err != nil {
		return fmt.Errorf("failed to parse sender identity key: %w", err)
	}
	expectedLockScript, err := brc29.LockForSelfDerived(ctx, sender, keyID, deriver)
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
