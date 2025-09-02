package validate

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// ValidateWalletInternalizeAction performs wallet-specific validation for internalize actions
func ValidateWalletInternalizeAction(ctx context.Context, keyDeriver *sdk.KeyDeriver, args *wdk.InternalizeActionArgs) error {
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

	address, err := brc29.AddressForSelf(brc29.PubHex(payment.SenderIdentityKey), keyID, keyDeriver)
	if err != nil {
		return fmt.Errorf("failed to create expected address: %w", err)
	}

	expectedLockScript, err := p2pkh.Lock(address)
	if err != nil {
		return fmt.Errorf("failed to create expected locking script: %w", err)
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
