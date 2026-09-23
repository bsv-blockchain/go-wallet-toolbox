package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// internalizeArgsFor builds wallet-payment internalize args around a caller-supplied BEEF.
func internalizeArgsFor(atomicBEEF []byte) wdk.InternalizeActionArgs {
	return wdk.InternalizeActionArgs{
		Tx: atomicBEEF,
		Outputs: []*wdk.InternalizeOutput{{
			OutputIndex: 0,
			Protocol:    wdk.WalletPaymentProtocol,
			PaymentRemittance: &wdk.WalletPayment{
				DerivationPrefix:  fixtures.DerivationPrefix,
				DerivationSuffix:  fixtures.DerivationSuffix,
				SenderIdentityKey: fixtures.UserIdentityKeyHex,
			},
		}},
		Description: "txid-only ancestor test",
	}
}

// beefWithTxidOnlyParent re-encodes tx so that parentTxID travels as a bare txid rather than in
// full — the shape a BRC-105 payer produces when the recipient has declared that ancestor
// through x-bsv-payment-known-txids.
func beefWithTxidOnlyParent(t *testing.T, tx *transaction.Transaction, parentTxID *chainhash.Hash) []byte {
	t.Helper()

	beef := transaction.NewBeefV2()
	beef.MergeTxidOnly(parentTxID)

	// Detach the linked parent, or MergeTransaction walks the SourceTransaction pointer and
	// puts it back in full — which is exactly what the payer is trying to avoid sending.
	for _, in := range tx.Inputs {
		in.SourceTransaction = nil
	}

	_, err := beef.MergeTransaction(tx)
	require.NoError(t, err)

	atomicBEEF, err := beef.AtomicBytes(tx.TxID())
	require.NoError(t, err)

	return atomicBEEF
}

// TestInternalizeActionAcceptsTxidOnlyAncestorAlreadyHeld is the end-to-end case for
// x-bsv-payment-known-txids: a payer omits an ancestor because the recipient said it already
// had it, and the recipient must then accept the payment.
//
// Both halves run against real storage. The parent is internalized first, which is what puts it
// in this storage's records; the child then arrives with that parent as a bare txid.
func TestInternalizeActionAcceptsTxidOnlyAncestorAlreadyHeld(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// given: a transaction this storage has internalized, and therefore holds
	parentSpec := testvectors.GivenTX().WithInput(1000).WithP2PKHOutput(900)
	parentTx := parentSpec.TX()

	parentBEEF, err := parentTx.AtomicBEEF(false)
	require.NoError(t, err)

	parentResult, err := activeStorage.InternalizeAction(t.Context(), testusers.Alice.AuthID(), internalizeArgsFor(parentBEEF))
	require.NoError(t, err, "the parent must internalize normally")
	require.True(t, parentResult.Accepted)

	// and: a payment spending it, with that parent omitted as a bare txid
	childSpec := testvectors.GivenTX().
		WithSender(testvectors.Bob).
		WithRecipient(testvectors.Charlie).
		WithInputFromUTXO(parentTx, 0).
		WithP2PKHOutput(800)

	childBEEF := beefWithTxidOnlyParent(t, childSpec.TX(), parentTx.TxID())

	// when:
	result, err := activeStorage.InternalizeAction(t.Context(), testusers.Alice.AuthID(), internalizeArgsFor(childBEEF))

	// then:
	require.NoError(t, err, "a declared ancestor must be restored from storage, not refused")
	assert.True(t, result.Accepted)
	assert.Equal(t, childSpec.TX().TxID().String(), result.TxID)
}

// TestInternalizeActionRefusesTxidOnlyAncestorNotHeld is the other half, and the one that keeps
// this from widening what internalize accepts: an ancestor this storage has never seen cannot
// be restored, so the payment is refused exactly as it was before. A payer has no business
// omitting a transaction the recipient never declared.
func TestInternalizeActionRefusesTxidOnlyAncestorNotHeld(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// given: a parent that is NEVER internalized, so storage has no record of it
	parentSpec := testvectors.GivenTX().WithInput(1000).WithP2PKHOutput(900)
	parentTx := parentSpec.TX()

	childSpec := testvectors.GivenTX().
		WithSender(testvectors.Bob).
		WithRecipient(testvectors.Charlie).
		WithInputFromUTXO(parentTx, 0).
		WithP2PKHOutput(800)

	childBEEF := beefWithTxidOnlyParent(t, childSpec.TX(), parentTx.TxID())

	// when:
	_, err := activeStorage.InternalizeAction(t.Context(), testusers.Alice.AuthID(), internalizeArgsFor(childBEEF))

	// then:
	require.Error(t, err, "an ancestor we cannot resolve must still be refused")
}
