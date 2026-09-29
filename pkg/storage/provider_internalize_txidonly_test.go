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

// TestInternalizeActionAcceptsTxidOnlyAncestorWhoseOwnRecordIsCollapsed is the SECOND
// generation, and the one a single declared ancestor does not cover.
//
// Once a payer starts declaring, the record storage keeps for each payment is itself collapsed:
// it holds the parent as a bare txid, because that is how the payment arrived. So the next
// payment declares an ancestor whose stored record is ALSO missing its ancestry, and restoring
// it is no longer one lookup — storage has to re-anchor what it handed back.
//
// That is where DirectSourcesOnly used to break the chain. It made the parent terminal by
// merging its raw tx alone, proof and input beef stripped; unanchorableSources then treated
// that proof-less parent as unanchored and demanded ITS parent, one generation further back
// than the payer ever sent.
//
// Observed on mainnet 2026-09-29 against pay-ovh: payment 2 of a self-chaining wallet was
// accepted and payment 3 was refused with "payment BEEF omits ancestors that are neither
// provided nor known to storage", naming a grandparent the payer had every right to omit. The
// payer had already broadcast, so it cost real satoshis and bought nothing.
func TestInternalizeActionAcceptsTxidOnlyAncestorWhoseOwnRecordIsCollapsed(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// given: a grandparent internalized in full, the way a first payment arrives
	// Each generation's sender must be the previous generation's recipient: WithP2PKHOutput
	// locks to the spec's recipient and WithInputFromUTXO unlocks with the spec's sender, so a
	// mismatched pair fails script verification long before any ancestry is resolved.
	grandparentSpec := testvectors.GivenTX().
		WithRecipient(testvectors.Bob).
		WithInput(1000).
		WithP2PKHOutput(900)
	grandparentTx := grandparentSpec.TX()

	grandparentBEEF, err := grandparentTx.AtomicBEEF(false)
	require.NoError(t, err)

	_, err = activeStorage.InternalizeAction(t.Context(), testusers.Alice.AuthID(), internalizeArgsFor(grandparentBEEF))
	require.NoError(t, err, "the grandparent must internalize normally")

	// and: a parent internalized with the grandparent declared away, so the record STORED for
	// the parent is itself collapsed. This is the generation that already worked.
	parentSpec := testvectors.GivenTX().
		WithSender(testvectors.Bob).
		WithRecipient(testvectors.Charlie).
		WithInputFromUTXO(grandparentTx, 0).
		WithP2PKHOutput(800)
	parentTx := parentSpec.TX()

	parentBEEF := beefWithTxidOnlyParent(t, parentTx, grandparentTx.TxID())

	_, err = activeStorage.InternalizeAction(t.Context(), testusers.Alice.AuthID(), internalizeArgsFor(parentBEEF))
	require.NoError(t, err, "the first declared generation must be accepted")

	// when: a child declares that parent, whose own stored record is collapsed
	childSpec := testvectors.GivenTX().
		WithSender(testvectors.Charlie).
		WithRecipient(testvectors.Bob).
		WithInputFromUTXO(parentTx, 0).
		WithP2PKHOutput(700)

	childBEEF := beefWithTxidOnlyParent(t, childSpec.TX(), parentTx.TxID())

	result, err := activeStorage.InternalizeAction(t.Context(), testusers.Alice.AuthID(), internalizeArgsFor(childBEEF))

	// then:
	require.NoError(t, err, "a declared ancestor must be restored even when its own stored record is collapsed")
	assert.True(t, result.Accepted)
	assert.Equal(t, childSpec.TX().TxID().String(), result.TxID)
}
