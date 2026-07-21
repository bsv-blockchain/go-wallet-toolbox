package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// ActionInternalizer is the wallet surface used for faucet bootstrap.
type ActionInternalizer interface {
	InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error)
}

// faucetDerivationParts builds a wallet-payment remittance matching the
// standard faucet address derivation used by the examples (AnyoneKey sender,
// fixed BRC-29 derivation prefix/suffix). Duplicated here so cmd/ does not
// import examples/.
func faucetDerivationParts() *sdk.Payment {
	const (
		defaultBase64Prefix = "SfKxPIJNgdI="
		defaultBase64Suffix = "NaGLC6fMH50="
	)
	prefix, err := base64.StdEncoding.DecodeString(defaultBase64Prefix)
	if err != nil {
		// Constants are valid base64; panic only if they are ever corrupted.
		panic(fmt.Errorf("decode faucet derivation prefix: %w", err))
	}
	suffix, err := base64.StdEncoding.DecodeString(defaultBase64Suffix)
	if err != nil {
		panic(fmt.Errorf("decode faucet derivation suffix: %w", err))
	}
	_, anyonePub := sdk.AnyoneKey()
	return &sdk.Payment{
		DerivationPrefix:  prefix,
		DerivationSuffix:  suffix,
		SenderIdentityKey: anyonePub,
	}
}

// BootstrapFaucet fetches BEEF for faucetTxID and internalizes output 0 as a
// wallet payment into the operator wallet (default basket).
func BootstrapFaucet(ctx context.Context, w ActionInternalizer, network defs.BSVNetwork, faucetTxID, originator string, logger *slog.Logger) error {
	if faucetTxID == "" {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}

	txIDHash, err := chainhash.NewHashFromHex(faucetTxID)
	if err != nil {
		return fmt.Errorf("invalid FAUCET_TXID %q: %w", faucetTxID, err)
	}

	srv := services.New(logger, defs.DefaultServicesConfig(network))
	logger.Info("fetching BEEF for faucet tx", "txid", faucetTxID, "network", network)
	beef, err := srv.GetBEEF(ctx, faucetTxID, nil)
	if err != nil {
		return fmt.Errorf("get BEEF for faucet txid %s: %w", faucetTxID, err)
	}

	atomicBeef, err := beef.AtomicBytes(txIDHash)
	if err != nil {
		return fmt.Errorf("atomic bytes for faucet txid %s: %w", faucetTxID, err)
	}

	args := sdk.InternalizeActionArgs{
		Tx: atomicBeef,
		Outputs: []sdk.InternalizeOutput{
			{
				OutputIndex:       0,
				Protocol:          sdk.InternalizeProtocolWalletPayment,
				PaymentRemittance: faucetDerivationParts(),
			},
		},
		Description: "throughput loadgen faucet bootstrap",
	}

	logger.Info("internalizing faucet funding", "txid", faucetTxID)
	if _, err := w.InternalizeAction(ctx, args, originator); err != nil {
		return fmt.Errorf("internalize faucet action: %w", err)
	}
	logger.Info("faucet funding internalized", "txid", faucetTxID)
	return nil
}
