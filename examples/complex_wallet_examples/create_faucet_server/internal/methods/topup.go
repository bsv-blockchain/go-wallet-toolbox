package methods

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox-faucet-server/internal/constants"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// TopUpInternalize validates tx belongs to faucet and internalizes it.
func TopUpInternalize(ctx context.Context, deps FaucetDeps, w sdk.Interface, txid string, outputIndex uint32) error {
	if txid == "" {
		return fmt.Errorf("txid is required")
	}

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

	addrStr, err := DeriveAddress(deps.FaucetPrivateKey, deps.Network)
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

	tx, err := transaction.NewTransactionFromBEEF(atomic)
	if err != nil {
		return fmt.Errorf("failed to parse tx: %w", err)
	}

	if outputIndex >= uint32(len(tx.Outputs)) || !tx.Outputs[outputIndex].LockingScript.Equals(expectedLock) { //nolint:gosec // safe: output count fits in uint32
		return fmt.Errorf("tx output[%d] does not match faucet address", outputIndex)
	}

	derivationPrefixBytes, err := base64.StdEncoding.DecodeString(constants.FaucetAddressKeyIDPrefix)
	if err != nil {
		return fmt.Errorf("failed to decode derivation prefix: %w", err)
	}

	derivationSuffixBytes, err := base64.StdEncoding.DecodeString(constants.FaucetAddressKeyIDSuffix)
	if err != nil {
		return fmt.Errorf("failed to decode derivation suffix: %w", err)
	}

	_, identityKey := sdk.AnyoneKey()
	internalizeArgs := sdk.InternalizeActionArgs{
		Tx: atomic,
		Outputs: []sdk.InternalizeOutput{{
			OutputIndex: outputIndex,
			Protocol:    "wallet payment",
			PaymentRemittance: &sdk.Payment{
				DerivationPrefix:  derivationPrefixBytes,
				DerivationSuffix:  derivationSuffixBytes,
				SenderIdentityKey: identityKey,
			},
		}},
		Description: "internalize from faucet",
	}

	if _, err := w.InternalizeAction(ctx, internalizeArgs, ""); err != nil {
		return fmt.Errorf("internalize failed: %w", err)
	}

	return nil
}
