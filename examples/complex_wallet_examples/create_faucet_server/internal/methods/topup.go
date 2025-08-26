package methods

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/constants"
	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/utils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// TopUpInternalize validates tx belongs to faucet and internalizes it.
func TopUpInternalize(ctx context.Context, deps FaucetDeps, _ *ec.PublicKey, w sdk.Interface, txid string, outputIndex uint32) error {
	if txid == "" {
		return fmt.Errorf("txid is required")
	}

	// Fetch BEEF for txid
	srv := services.New(slog.Default(), defs.DefaultServicesConfig(deps.Network))
	beef, err := srv.GetBEEF(ctx, txid, nil)
	if err != nil {
		return fmt.Errorf("failed to get BEEF: %w", err)
	}

	h, err := chainhash.NewHashFromHex(txid)
	if err != nil {
		return fmt.Errorf("invalid txid: %w", err)
	}
	atomic, err := beef.AtomicBytes(h)
	if err != nil {
		return fmt.Errorf("failed to get atomic bytes: %w", err)
	}

	// Build expected locking script for faucet BRC-29 address (P2PKH)
	addrStr, err := DeriveAddress(deps.FaucetKeyHex, deps.Network)
	if err != nil {
		return fmt.Errorf("failed to derive faucet address: %w", err)
	}
	addr, err := script.NewAddressFromString(addrStr)
	if err != nil {
		return fmt.Errorf("failed to parse faucet address: %w", err)
	}
	expectedLock, err := p2pkh.Lock(addr)
	if err != nil {
		return fmt.Errorf("failed to create locking script: %w", err)
	}

	// Parse tx and validate the specified output matches faucet locking script
	tx, err := transaction.NewTransactionFromBEEF(atomic)
	if err != nil {
		return fmt.Errorf("failed to parse tx: %w", err)
	}
	if int(outputIndex) >= len(tx.Outputs) || !tx.Outputs[outputIndex].LockingScript.Equals(expectedLock) {
		return fmt.Errorf("tx output[%d] does not match faucet address", outputIndex)
	}

	// Decode derivation prefix/suffix from base64 (same as manual_tests/internal/internalize.go)
	derivationPrefixBytes, err := utils.BytesFromBase64(constants.DefaultBase64Prefix)
	if err != nil {
		return fmt.Errorf("failed to convert derivation prefix from base64: %w", err)
	}

	derivationSuffixBytes, err := utils.BytesFromBase64(constants.DefaultBase64Suffix)
	if err != nil {
		return fmt.Errorf("failed to convert derivation suffix from base64: %w", err)
	}

	faucetPriv, err := ec.PrivateKeyFromHex(deps.FaucetKeyHex)
	if err != nil {
		return fmt.Errorf("failed to parse faucet private key: %w", err)
	}
	faucetPub := faucetPriv.PubKey()
	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: atomic,
		Outputs: []sdk.InternalizeOutput{{
			OutputIndex: outputIndex,
			Protocol:    "wallet payment",
			PaymentRemittance: &sdk.Payment{
				DerivationPrefix:  derivationPrefixBytes,
				DerivationSuffix:  derivationSuffixBytes,
				SenderIdentityKey: faucetPub,
			},
		}},
		Description: "internalize from faucet",
	}

	if _, err := w.InternalizeAction(ctx, internalizeArgs, ""); err != nil {
		return fmt.Errorf("internalize failed: %w", err)
	}
	return nil
}
