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

// InternalizeActioner is the wallet surface for InternalizeAction.
type InternalizeActioner interface {
	InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error)
}

// AtomicBeefer fetches atomic BEEF bytes for a txid when AtomicTxHex is empty.
// Production uses services.GetBEEF; tests inject a fake via WithAtomicBeefSource.
type AtomicBeefer interface {
	AtomicBeef(ctx context.Context, txID string) ([]byte, error)
}

// InternalizeRequest is the body accepted by POST /api/funding/internalize.
type InternalizeRequest struct {
	// AtomicTxHex is preferred: atomic BEEF or raw tx hex from WalletClient.
	AtomicTxHex string `json:"atomic_tx_hex"`
	// TxID is used when AtomicTxHex is empty (fetch BEEF via services / AtomicBeefer).
	TxID string `json:"txid"`
	// OutputIndex defaults to 0.
	OutputIndex uint32 `json:"output_index"`
}

// InternalizeParams groups the arguments for Internalize.
type InternalizeParams struct {
	Wallet          InternalizeActioner
	Network         defs.BSVNetwork
	ExpectedAddress string
	Request         InternalizeRequest
	Originator      string
	Logger          *slog.Logger
}

type internalizeOptions struct {
	beefSource AtomicBeefer
}

// InternalizeOption configures Internalize (optional; production callers omit).
type InternalizeOption func(*internalizeOptions)

// WithAtomicBeefSource overrides the default services.GetBEEF path (unit tests).
func WithAtomicBeefSource(src AtomicBeefer) InternalizeOption {
	return func(o *internalizeOptions) {
		o.beefSource = src
	}
}

// Internalize credits a WalletClient (or external) payment into the operator default basket.
// Prefers AtomicTxHex; otherwise fetches BEEF by TxID. Remittance is always AnyoneKey wallet-payment.
func Internalize(ctx context.Context, p InternalizeParams, opts ...InternalizeOption) error {
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if p.Wallet == nil {
		return fmt.Errorf("wallet is required")
	}

	cfg := internalizeOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}

	atomic, err := loadAtomicTx(ctx, p, cfg, logger)
	if err != nil {
		return err
	}

	outputIndex, err := resolveOutputIndex(atomic, p.Request.OutputIndex, p.ExpectedAddress, logger)
	if err != nil {
		return err
	}

	return submitInternalize(ctx, p, atomic, outputIndex, logger)
}

func loadAtomicTx(ctx context.Context, p InternalizeParams, cfg internalizeOptions, logger *slog.Logger) ([]byte, error) {
	if p.Request.AtomicTxHex == "" && p.Request.TxID == "" {
		return nil, fmt.Errorf("atomic_tx_hex or txid is required")
	}
	if p.Request.AtomicTxHex != "" {
		atomic, err := hex.DecodeString(p.Request.AtomicTxHex)
		if err != nil {
			return nil, fmt.Errorf("decode atomic_tx_hex: %w", err)
		}
		return atomic, nil
	}
	src := cfg.beefSource
	if src == nil {
		src = servicesAtomicBeefSource{network: p.Network, logger: logger}
	}
	return src.AtomicBeef(ctx, p.Request.TxID)
}

func resolveOutputIndex(atomic []byte, preferred uint32, expectedAddress string, logger *slog.Logger) (uint32, error) {
	if expectedAddress == "" {
		return preferred, nil
	}
	resolved, err := resolvePaymentOutputIndex(atomic, preferred, expectedAddress)
	if err != nil {
		return 0, err
	}
	if resolved != preferred {
		logger.Info(
			"funding payment vout differs from requested index",
			"requested", preferred,
			"resolved", resolved,
		)
	}
	return resolved, nil
}

func submitInternalize(
	ctx context.Context,
	p InternalizeParams,
	atomic []byte,
	outputIndex uint32,
	logger *slog.Logger,
) error {
	remittance, err := AnyonePaymentRemittance()
	if err != nil {
		return err
	}

	args := sdk.InternalizeActionArgs{
		Tx: atomic,
		Outputs: []sdk.InternalizeOutput{
			{
				OutputIndex:       outputIndex,
				Protocol:          sdk.InternalizeProtocolWalletPayment,
				PaymentRemittance: remittance,
			},
		},
		Description: "throughput dashboard WalletClient top-up",
	}

	if _, err := p.Wallet.InternalizeAction(ctx, args, p.Originator); err != nil {
		return fmt.Errorf("internalize: %w", err)
	}

	logger.Info("funding internalized", "output_index", outputIndex, "txid", logTxID(p.Request, atomic))
	return nil
}

func logTxID(req InternalizeRequest, atomic []byte) string {
	if req.TxID != "" {
		return req.TxID
	}
	parsed, parseErr := parseTx(atomic)
	if parseErr != nil || parsed == nil {
		return ""
	}
	if h := parsed.TxID(); h != nil {
		return h.String()
	}
	return ""
}

// servicesAtomicBeefSource is the production BEEF fetcher via pkg/services.
type servicesAtomicBeefSource struct {
	network defs.BSVNetwork
	logger  *slog.Logger
}

func (s servicesAtomicBeefSource) AtomicBeef(ctx context.Context, txID string) ([]byte, error) {
	txIDHash, err := chainhash.NewHashFromHex(txID)
	if err != nil {
		return nil, fmt.Errorf("invalid txid: %w", err)
	}
	srv := services.New(s.logger, defs.DefaultServicesConfig(s.network))
	beef, err := srv.GetBEEF(ctx, txID, nil)
	if err != nil {
		return nil, fmt.Errorf("get BEEF for %s: %w", txID, err)
	}
	atomic, err := beef.AtomicBytes(txIDHash)
	if err != nil {
		return nil, fmt.Errorf("atomic bytes: %w", err)
	}
	return atomic, nil
}

func parseTx(atomic []byte) (*transaction.Transaction, error) {
	tx, err := transaction.NewTransactionFromBEEF(atomic)
	if err != nil {
		return transaction.NewTransactionFromBytes(atomic)
	}
	return tx, nil
}

// resolvePaymentOutputIndex returns the vout that pays expectedAddress.
// Prefer preferredIndex when it matches; otherwise scan all outputs (wallets
// commonly place change at vout 0 and the deposit later). If the tx cannot be
// parsed, preferredIndex is returned unchanged so InternalizeAction can try.
func resolvePaymentOutputIndex(atomic []byte, preferredIndex uint32, expectedAddress string) (uint32, error) {
	addr, err := script.NewAddressFromString(expectedAddress)
	if err != nil {
		return 0, fmt.Errorf("parse expected address: %w", err)
	}
	expectedLock, err := p2pkh.Lock(addr)
	if err != nil {
		return 0, fmt.Errorf("expected lock: %w", err)
	}

	tx, err := parseTx(atomic)
	if err != nil {
		// Skip strict validation if we cannot parse; InternalizeAction will fail if bad.
		return preferredIndex, nil
	}
	if len(tx.Outputs) == 0 {
		return 0, fmt.Errorf("transaction has no outputs")
	}

	pays := func(i int) bool {
		out := tx.Outputs[i]
		return out != nil && out.LockingScript != nil && out.LockingScript.Equals(expectedLock)
	}

	if int(preferredIndex) < len(tx.Outputs) && pays(int(preferredIndex)) {
		return preferredIndex, nil
	}

	for i := range tx.Outputs {
		if pays(i) {
			return uint32(i), nil //nolint:gosec // output index fits uint32
		}
	}

	return 0, fmt.Errorf(
		"no output pays operator deposit address %s (tx has %d outputs; requested vout %d often is wallet change — payment may be a later vout)",
		expectedAddress, len(tx.Outputs), preferredIndex,
	)
}
