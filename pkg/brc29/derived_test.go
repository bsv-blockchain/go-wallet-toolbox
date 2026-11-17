package brc29_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script/interpreter"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
)

// sdkSigner adapts go-sdk's KeyDeriver to the private-key-free BRC-29 seam.
type sdkSigner struct {
	kd *sdk.KeyDeriver
}

func (s sdkSigner) IdentityKey() *ec.PublicKey { return s.kd.IdentityKey() }

func (s sdkSigner) DerivePublicKey(_ context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, forSelf bool) (*ec.PublicKey, error) {
	return s.kd.DerivePublicKey(protocol, keyID, counterparty, forSelf)
}

func (s sdkSigner) Sign(_ context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, digest []byte) (*ec.Signature, error) {
	priv, err := s.kd.DerivePrivateKey(protocol, keyID, counterparty)
	if err != nil {
		return nil, err
	}
	return priv.Sign(digest)
}

func TestDerivedLockMatchesKeyDeriver(t *testing.T) {
	sender, err := ec.PrivateKeyFromHex(senderPrivateKeyHex)
	require.NoError(t, err)
	recipient, err := ec.PrivateKeyFromHex(recipientPrivateKeyHex)
	require.NoError(t, err)
	senderD := sdkSigner{kd: sdk.NewKeyDeriver(sender)}
	recipientD := sdkSigner{kd: sdk.NewKeyDeriver(recipient)}

	wantCounterparty, err := brc29.LockForCounterparty(senderD.kd, keyID, recipient.PubKey())
	require.NoError(t, err)
	gotCounterparty, err := brc29.LockForCounterpartyDerived(t.Context(), senderD, keyID, recipient.PubKey())
	require.NoError(t, err)
	require.Equal(t, wantCounterparty.String(), gotCounterparty.String())

	wantSelf, err := brc29.LockForSelf(sender.PubKey(), keyID, recipientD.kd)
	require.NoError(t, err)
	gotSelf, err := brc29.LockForSelfDerived(t.Context(), sender.PubKey(), keyID, recipientD)
	require.NoError(t, err)
	require.Equal(t, wantSelf.String(), gotSelf.String())
}

func TestDerivedUnlockSpendsBRC29Output(t *testing.T) {
	for _, combined := range []bool{false, true} {
		t.Run(fmt.Sprintf("combined=%v", combined), func(t *testing.T) { testDerivedUnlock(t, combined) })
	}
}

type combinedSDKSigner struct {
	sdkSigner
	calls int
}

func (s *combinedSDKSigner) DerivePublicKey(context.Context, sdk.Protocol, string, sdk.Counterparty, bool) (*ec.PublicKey, error) {
	return nil, fmt.Errorf("separate public derivation must not be called")
}
func (s *combinedSDKSigner) SignWithPublicKey(ctx context.Context, p sdk.Protocol, id string, cp sdk.Counterparty, digest []byte) (*ec.Signature, *ec.PublicKey, error) {
	s.calls++
	sig, err := s.sdkSigner.Sign(ctx, p, id, cp, digest)
	if err != nil {
		return nil, nil, err
	}
	pub, err := s.kd.DerivePublicKey(p, id, cp, true)
	return sig, pub, err
}

func testDerivedUnlock(t *testing.T, combined bool) {
	t.Helper()
	sender, err := ec.PrivateKeyFromHex(senderPrivateKeyHex)
	require.NoError(t, err)
	recipient, err := ec.PrivateKeyFromHex(recipientPrivateKeyHex)
	require.NoError(t, err)

	lockingScript, err := brc29.LockForCounterpartyDerived(t.Context(), sdkSigner{kd: sdk.NewKeyDeriver(sender)}, keyID, recipient.PubKey())
	require.NoError(t, err)
	var signer brc29.DerivedSigner = sdkSigner{kd: sdk.NewKeyDeriver(recipient)}
	fast := &combinedSDKSigner{sdkSigner: sdkSigner{kd: sdk.NewKeyDeriver(recipient)}}
	if combined {
		signer = fast
	}
	unlocker, err := brc29.UnlockDerived(t.Context(), sender.PubKey(), keyID, signer)
	require.NoError(t, err)
	require.EqualValues(t, 106, unlocker.EstimateLength(transaction.NewTransaction(), 0))

	prevTxID, err := chainhash.NewHashFromHex("64faeaa2e3cbadaf82d8fa8c7ded508cb043c5d101671f43c084be2ac6163148")
	require.NoError(t, err)
	tx := transaction.NewTransaction()
	require.NoError(t, tx.AddOpReturnOutput([]byte("anything")))
	require.NoError(t, tx.AddInputsFromUTXOs(&transaction.UTXO{
		TxID:                    prevTxID,
		Vout:                    0,
		Satoshis:                1000,
		LockingScript:           lockingScript,
		UnlockingScriptTemplate: unlocker,
	}))
	require.NoError(t, tx.Sign())
	if combined {
		require.Equal(t, 1, fast.calls)
	}
	require.NoError(t, interpreter.NewEngine().Execute(
		interpreter.WithTx(tx, 0, tx.Inputs[0].SourceTxOutput()),
		interpreter.WithAfterGenesis(),
		interpreter.WithForkID(),
	))
}
