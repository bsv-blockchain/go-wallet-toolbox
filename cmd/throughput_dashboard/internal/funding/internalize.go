package funding

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

// ActionInternalizer is the wallet surface for InternalizeAction.
type ActionInternalizer interface {
	InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error)
}

// InternalizeRequest is the body accepted by POST /api/funding/internalize.
type InternalizeRequest struct {
	// AtomicTxHex is preferred: atomic BEEF or raw tx hex from WalletClient.
	AtomicTxHex string `json:"atomic_tx_hex"`
	// TxID is used when AtomicTxHex is empty (fetch BEEF via services).
	TxID string `json:"txid"`
	// OutputIndex defaults to 0.
	OutputIndex uint32 `json:"output_index"`
}

// Internalize credits a WalletClient (or external) payment into the operator default basket.
func Internalize(
	ctx context.Context,
	w ActionInternalizer,
	network defs.BSVNetwork,
	expectedAddress string,
	req InternalizeRequest,
	originator string,
	logger *slog.Logger,
) error {
	if logger == nil {
		logger = slog.Default()
	}
	if req.AtomicTxHex == "" && req.TxID == "" {
		return fmt.Errorf("atomic_tx_hex or txid is required")
	}

	var atomic []byte
	var err error
	if req.AtomicTxHex != "" {
		atomic, err = hex.DecodeString(req.AtomicTxHex)
		if err != nil {
			return fmt.Errorf("decode atomic_tx_hex: %w", err)
		}
	} else {
		txIDHash, err := chainhash.NewHashFromHex(req.TxID)
		if err != nil {
			return fmt.Errorf("invalid txid: %w", err)
		}
		srv := services.New(logger, defs.DefaultServicesConfig(network))
		beef, err := srv.GetBEEF(ctx, req.TxID, nil)
		if err != nil {
			return fmt.Errorf("get BEEF for %s: %w", req.TxID, err)
		}
		atomic, err = beef.AtomicBytes(txIDHash)
		if err != nil {
			return fmt.Errorf("atomic bytes: %w", err)
		}
	}

	// Validate the claimed output pays the operator deposit address when possible.
	if expectedAddress != "" {
		if err := validateOutputPaysAddress(atomic, req.OutputIndex, expectedAddress); err != nil {
			return err
		}
	}

	remittance, err := AnyonePaymentRemittance()
	if err != nil {
		return err
	}

	args := sdk.InternalizeActionArgs{
		Tx: atomic,
		Outputs: []sdk.InternalizeOutput{
			{
				OutputIndex:       req.OutputIndex,
				Protocol:          sdk.InternalizeProtocolWalletPayment,
				PaymentRemittance: remittance,
			},
		},
		Description: "throughput dashboard WalletClient top-up",
	}

	if _, err := w.InternalizeAction(ctx, args, originator); err != nil {
		return fmt.Errorf("internalize: %w", err)
	}
	logger.Info("funding internalized", "output_index", req.OutputIndex, "txid", req.TxID)
	return nil
}

func validateOutputPaysAddress(atomic []byte, outputIndex uint32, expectedAddress string) error {
	addr, err := script.NewAddressFromString(expectedAddress)
	if err != nil {
		return fmt.Errorf("parse expected address: %w", err)
	}
	expectedLock, err := p2pkh.Lock(addr)
	if err != nil {
		return fmt.Errorf("expected lock: %w", err)
	}

	// Try BEEF first, then raw tx.
	tx, err := transaction.NewTransactionFromBEEF(atomic)
	if err != nil {
		tx, err = transaction.NewTransactionFromBytes(atomic)
		if err != nil {
			// Skip strict validation if we cannot parse; InternalizeAction will fail if bad.
			return nil
		}
	}
	if outputIndex >= uint32(len(tx.Outputs)) {
		return fmt.Errorf("output_index %d out of range (tx has %d outputs)", outputIndex, len(tx.Outputs))
	}
	if !tx.Outputs[outputIndex].LockingScript.Equals(expectedLock) {
		return fmt.Errorf("output[%d] does not pay operator deposit address", outputIndex)
	}
	return nil
}
