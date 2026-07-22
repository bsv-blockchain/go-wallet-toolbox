package funding_test

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/funding"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/stretchr/testify/require"
)

type fakeInternalizer struct {
	lastArgs       sdk.InternalizeActionArgs
	lastOriginator string
	calls          int
	err            error
}

func (f *fakeInternalizer) InternalizeAction(
	_ context.Context,
	args sdk.InternalizeActionArgs,
	originator string,
) (*sdk.InternalizeActionResult, error) {
	f.calls++
	f.lastArgs = args
	f.lastOriginator = originator
	if f.err != nil {
		return nil, f.err
	}
	return &sdk.InternalizeActionResult{Accepted: true}, nil
}

type fakeBeefSource struct {
	atomic []byte
	err    error
	calls  int
	lastID string
}

func (f *fakeBeefSource) AtomicBeef(_ context.Context, txID string) ([]byte, error) {
	f.calls++
	f.lastID = txID
	if f.err != nil {
		return nil, f.err
	}
	return f.atomic, nil
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func doInternalize(p funding.InternalizeParams, opts ...funding.InternalizeOption) error {
	return funding.Internalize(context.Background(), p, opts...)
}

func params(
	w funding.InternalizeActioner,
	addr string,
	req funding.InternalizeRequest,
) funding.InternalizeParams {
	return funding.InternalizeParams{
		Wallet:          w,
		Network:         defs.NetworkMainnet,
		ExpectedAddress: addr,
		Request:         req,
		Originator:      "origin",
		Logger:          silentLogger(),
	}
}

func p2pkhLock(t *testing.T, address string) *script.Script {
	t.Helper()
	addr, err := script.NewAddressFromString(address)
	require.NoError(t, err)
	lock, err := p2pkh.Lock(addr)
	require.NoError(t, err)
	return lock
}

func p2pkhRawTxHex(t *testing.T, address string) string {
	t.Helper()
	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      50_000,
		LockingScript: p2pkhLock(t, address),
	})
	return hex.EncodeToString(tx.Bytes())
}

// changeThenPaymentRawTxHex models a typical local-wallet createAction layout:
// vout 0 = change back to the payer, vout 1 = payment to the deposit address.
func changeThenPaymentRawTxHex(t *testing.T, changeAddress, depositAddress string) string {
	t.Helper()
	tx := transaction.NewTransaction()
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      10_000,
		LockingScript: p2pkhLock(t, changeAddress),
	})
	tx.AddOutput(&transaction.TransactionOutput{
		Satoshis:      50_000,
		LockingScript: p2pkhLock(t, depositAddress),
	})
	return hex.EncodeToString(tx.Bytes())
}

func TestInternalizeRequiresTxHexOrTxID(t *testing.T) {
	w := &fakeInternalizer{}
	err := doInternalize(params(w, "", funding.InternalizeRequest{}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "atomic_tx_hex or txid")
	require.Equal(t, 0, w.calls)
}

func TestInternalizeNilWallet(t *testing.T) {
	err := doInternalize(params(nil, "", funding.InternalizeRequest{AtomicTxHex: "00"}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wallet")
}

func TestInternalizeBadAtomicHex(t *testing.T) {
	w := &fakeInternalizer{}
	err := doInternalize(params(w, "", funding.InternalizeRequest{AtomicTxHex: "zz"}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode atomic_tx_hex")
	require.Equal(t, 0, w.calls)
}

func TestInternalizeAtomicPathSuccess(t *testing.T) {
	priv := testOperatorPriv(t)
	info, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 0)
	require.NoError(t, err)

	rawHex := p2pkhRawTxHex(t, info.Address)
	w := &fakeInternalizer{}

	p := params(w, info.Address, funding.InternalizeRequest{AtomicTxHex: rawHex, OutputIndex: 0})
	p.Originator = "throughput-dashboard.local"
	err = doInternalize(p)
	require.NoError(t, err)
	require.Equal(t, 1, w.calls)
	require.Equal(t, "throughput-dashboard.local", w.lastOriginator)
	require.Equal(t, sdk.InternalizeProtocolWalletPayment, w.lastArgs.Outputs[0].Protocol)
	require.NotNil(t, w.lastArgs.Outputs[0].PaymentRemittance)

	wantRemit, err := funding.AnyonePaymentRemittance()
	require.NoError(t, err)
	got := w.lastArgs.Outputs[0].PaymentRemittance
	require.Equal(t, wantRemit.DerivationPrefix, got.DerivationPrefix)
	require.Equal(t, wantRemit.DerivationSuffix, got.DerivationSuffix)
	require.Equal(t, wantRemit.SenderIdentityKey.ToDERHex(), got.SenderIdentityKey.ToDERHex())

	decoded, err := hex.DecodeString(rawHex)
	require.NoError(t, err)
	require.Equal(t, decoded, w.lastArgs.Tx)
}

func TestInternalizePrefersAtomicOverTxID(t *testing.T) {
	priv := testOperatorPriv(t)
	info, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 0)
	require.NoError(t, err)

	rawHex := p2pkhRawTxHex(t, info.Address)
	w := &fakeInternalizer{}
	beef := &fakeBeefSource{atomic: []byte("should-not-be-used")}

	err = doInternalize(
		params(w, info.Address, funding.InternalizeRequest{
			AtomicTxHex: rawHex,
			TxID:        "aabbccdd", // ignored when atomic present
		}),
		funding.WithAtomicBeefSource(beef),
	)
	require.NoError(t, err)
	require.Equal(t, 0, beef.calls)
	require.Equal(t, 1, w.calls)
}

func TestInternalizeTxIDPathUsesBeefSource(t *testing.T) {
	priv := testOperatorPriv(t)
	info, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 0)
	require.NoError(t, err)

	raw, err := hex.DecodeString(p2pkhRawTxHex(t, info.Address))
	require.NoError(t, err)

	w := &fakeInternalizer{}
	beef := &fakeBeefSource{atomic: raw}
	const txid = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	err = doInternalize(
		params(w, info.Address, funding.InternalizeRequest{TxID: txid, OutputIndex: 0}),
		funding.WithAtomicBeefSource(beef),
	)
	require.NoError(t, err)
	require.Equal(t, 1, beef.calls)
	require.Equal(t, txid, beef.lastID)
	require.Equal(t, 1, w.calls)
	require.Equal(t, raw, w.lastArgs.Tx)
}

func TestInternalizeTxIDBeefSourceError(t *testing.T) {
	w := &fakeInternalizer{}
	beef := &fakeBeefSource{err: errors.New("network down")}

	err := doInternalize(
		params(w, "", funding.InternalizeRequest{TxID: "aa"}),
		funding.WithAtomicBeefSource(beef),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "network down")
	require.Equal(t, 0, w.calls)
}

func TestInternalizeRejectsWrongAddress(t *testing.T) {
	priv := testOperatorPriv(t)
	info, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 0)
	require.NoError(t, err)

	// Build a payment to a different address.
	otherPriv, err := ec.NewPrivateKey()
	require.NoError(t, err)
	otherInfo, err := funding.DeriveInfo(otherPriv, defs.NetworkMainnet, 0)
	require.NoError(t, err)

	rawHex := p2pkhRawTxHex(t, otherInfo.Address)
	w := &fakeInternalizer{}

	err = doInternalize(params(w, info.Address, funding.InternalizeRequest{AtomicTxHex: rawHex}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no output pays operator deposit address")
	require.Equal(t, 0, w.calls)
}

func TestInternalizeAutoDetectsPaymentAfterChangeOutput(t *testing.T) {
	// Local wallets typically put change at vout 0 and the payment later.
	// Requesting vout 0 (default) must still resolve to the deposit output.
	opPriv := testOperatorPriv(t)
	info, err := funding.DeriveInfo(opPriv, defs.NetworkMainnet, 0)
	require.NoError(t, err)

	changePriv, err := ec.NewPrivateKey()
	require.NoError(t, err)
	changeInfo, err := funding.DeriveInfo(changePriv, defs.NetworkMainnet, 0)
	require.NoError(t, err)

	rawHex := changeThenPaymentRawTxHex(t, changeInfo.Address, info.Address)
	w := &fakeInternalizer{}

	err = doInternalize(params(w, info.Address, funding.InternalizeRequest{AtomicTxHex: rawHex, OutputIndex: 0})) // wrong preferred index
	require.NoError(t, err)
	require.Equal(t, 1, w.calls)
	require.Equal(t, uint32(1), w.lastArgs.Outputs[0].OutputIndex)
}

func TestInternalizeResolvesWrongPreferredIndexWhenPaymentExists(t *testing.T) {
	priv := testOperatorPriv(t)
	info, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 0)
	require.NoError(t, err)

	rawHex := p2pkhRawTxHex(t, info.Address)
	w := &fakeInternalizer{}

	// Preferred index out of range, but deposit is on vout 0 — auto-resolve.
	err = doInternalize(params(w, info.Address, funding.InternalizeRequest{AtomicTxHex: rawHex, OutputIndex: 5}))
	require.NoError(t, err)
	require.Equal(t, 1, w.calls)
	require.Equal(t, uint32(0), w.lastArgs.Outputs[0].OutputIndex)
}

func TestInternalizeSkipsValidationWhenUnparseable(t *testing.T) {
	// Unparseable atomic bytes: validation is skipped; wallet is still called.
	w := &fakeInternalizer{}
	err := doInternalize(params(w, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", funding.InternalizeRequest{AtomicTxHex: "00"})) // valid address form
	require.NoError(t, err)
	require.Equal(t, 1, w.calls)
}

func TestInternalizePropagatesWalletError(t *testing.T) {
	priv := testOperatorPriv(t)
	info, err := funding.DeriveInfo(priv, defs.NetworkMainnet, 0)
	require.NoError(t, err)

	rawHex := p2pkhRawTxHex(t, info.Address)
	w := &fakeInternalizer{err: errors.New("wallet reject")}

	err = doInternalize(params(w, info.Address, funding.InternalizeRequest{AtomicTxHex: rawHex}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "internalize")
	require.Contains(t, err.Error(), "wallet reject")
}

func TestInternalizeInvalidExpectedAddress(t *testing.T) {
	w := &fakeInternalizer{}
	err := doInternalize(params(w, "not-an-address", funding.InternalizeRequest{AtomicTxHex: "00"}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse expected address")
	require.Equal(t, 0, w.calls)
}

