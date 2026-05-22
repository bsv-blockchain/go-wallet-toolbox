package assembler_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/assembler"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
)

func TestToAtomicBEEF_RemovesUnsignedTx(t *testing.T) {
	// Create a transaction
	tx := &transaction.Transaction{Version: 1}
	unsignedID := tx.TxID()

	// Create a BEEF and add the unsigned transaction to it
	beef := transaction.NewBeef()
	_, err := beef.MergeRawTx(tx.Bytes(), nil)
	require.NoError(t, err)
	require.Contains(t, beef.Transactions, *unsignedID)

	// Create AssembledTransaction from pending action
	pAction := &pending.SignAction{
		Tx:        *tx,
		InputBEEF: beef,
	}
	assembled := assembler.NewAssembledTxFromPendingSignAction(pAction)

	// Now "sign" the transaction (simulated by changing something that changes TXID)
	assembled.LockTime = 12345
	signedID := assembled.TxID()
	require.NotEqual(t, unsignedID, signedID)

	// Generate BEEF
	finalBeef, err := assembled.ToAtomicBEEF(true)
	require.NoError(t, err)

	// Final BEEF should not contain the unsigned TXID
	require.Contains(t, finalBeef.Transactions, *signedID)
	require.NotContains(t, finalBeef.Transactions, *unsignedID, "Final BEEF should NOT contain the unsigned transaction ID")
}

// TestAtomicBEEF_PreservesBumpIndexAcrossMultipleBumps is a regression test for a bug where
// mergeSourceTxIntoBEEF would call beef.MergeRawTx on a tx that was already present in the BEEF
// with the correct BumpIndex (set by MergeBeef). go-sdk's MergeRawTx forgets to assign
// BeefTx.BumpIndex, so the serialized BEEF would always encode BumpIndex=0 for those txs,
// causing the recipient to attach the wrong merkle proof when the BEEF holds 2+ BUMPs.
func TestAtomicBEEF_PreservesBumpIndexAcrossMultipleBumps(t *testing.T) {
	tru := true

	h := func(hex string) *chainhash.Hash {
		t.Helper()
		hash, err := chainhash.NewHashFromHex(hex)
		require.NoError(t, err)
		return hash
	}

	// grandparentTx and parentTx are standalone mined txs (no inputs needed for serialization).
	grandparentTx := &transaction.Transaction{Version: 1, LockTime: 111}
	grandparentTxID := grandparentTx.TxID()

	parentTx := &transaction.Transaction{Version: 1, LockTime: 222}
	parentTx.AddInput(&transaction.TransactionInput{
		SourceTXID:        grandparentTxID,
		SourceTransaction: grandparentTx,
		SourceTxOutIndex:  0,
	})
	parentTxID := parentTx.TxID()

	// Build MerklePaths whose leaf hashes are the actual computed txids so that
	// tryToValidateBumpIndex (inside MergeTransaction) doesn't discard them.
	sibling0 := h("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	// bump0 proves grandparentTx at block height 1000.
	bump0 := transaction.NewMerklePath(1000, [][]*transaction.PathElement{
		{
			{Offset: 0, Txid: &tru, Hash: grandparentTxID},
			{Offset: 1, Hash: sibling0},
		},
		{
			{Offset: 0, Hash: h("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")},
		},
	})
	grandparentTx.MerklePath = bump0

	sibling1 := h("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	// bump1 proves parentTx at block height 2000 — will land at index 1 in the dest BEEF.
	bump1 := transaction.NewMerklePath(2000, [][]*transaction.PathElement{
		{
			{Offset: 0, Txid: &tru, Hash: parentTxID},
			{Offset: 1, Hash: sibling1},
		},
		{
			{Offset: 0, Hash: h("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")},
		},
	})
	parentTx.MerklePath = bump1

	// Build the input BEEF. Merge grandparent first so bump0 lands at index 0;
	// then merge parent so bump1 lands at index 1.
	inputBEEF := transaction.NewBeef()
	_, err := inputBEEF.MergeTransaction(grandparentTx)
	require.NoError(t, err)
	_, err = inputBEEF.MergeTransaction(parentTx)
	require.NoError(t, err)
	require.Len(t, inputBEEF.BUMPs, 2, "input BEEF must have exactly 2 BUMPs (one per block)")

	// Record which BEEF index proves parentTx — this must survive round-tripping.
	inputParentEntry, ok := inputBEEF.Transactions[*parentTxID]
	require.True(t, ok, "parentTx must be in inputBEEF")
	require.Equal(t, transaction.RawTxAndBumpIndex, inputParentEntry.DataFormat,
		"parentTx must carry RawTxAndBumpIndex in inputBEEF (MerklePath leaf hash must match txid)")
	expectedBumpIndex := inputParentEntry.BumpIndex
	require.Equal(t, 1, expectedBumpIndex, "parentTx should be proven by BUMPs[1]")

	// The assembled ("unsigned") tx spends parentTx.
	unsignedTx := &transaction.Transaction{Version: 1, LockTime: 333}
	unsignedTx.AddInput(&transaction.TransactionInput{
		SourceTXID:        parentTxID,
		SourceTransaction: parentTx,
		SourceTxOutIndex:  0,
	})
	unsignedTxID := unsignedTx.TxID()

	// Simulate adding unsigned tx to inputBEEF as CreateAction would do.
	_, err = inputBEEF.MergeRawTx(unsignedTx.Bytes(), nil)
	require.NoError(t, err)

	pAction := &pending.SignAction{
		Tx:        *unsignedTx,
		InputBEEF: inputBEEF,
	}
	assembled := assembler.NewAssembledTxFromPendingSignAction(pAction)

	// Simulate signing by mutating LockTime (changes the TxID so the unsigned entry is removed).
	assembled.LockTime = 999
	signedID := assembled.TxID()
	require.NotEqual(t, unsignedTxID, signedID)

	// Build the atomic BEEF bytes.
	atomicBytes, err := assembled.AtomicBEEF(true)
	require.NoError(t, err)

	// Round-trip through serialization — this is where the bug manifested.
	roundTripped, _, err := transaction.NewBeefFromAtomicBytes(atomicBytes)
	require.NoError(t, err)

	parentEntry, ok := roundTripped.Transactions[*parentTxID]
	require.True(t, ok, "parentTx must be present in the round-tripped BEEF")

	assert.Equal(t, transaction.RawTxAndBumpIndex, parentEntry.DataFormat,
		"parentTx DataFormat must be RawTxAndBumpIndex after round-trip")
	assert.Equal(t, expectedBumpIndex, parentEntry.BumpIndex,
		"parentTx BumpIndex must survive round-trip serialization; "+
			"without the fix, mergeSourceTxIntoBEEF clobbered it to 0 via MergeRawTx")

	require.NotNil(t, parentEntry.Transaction.MerklePath,
		"parentTx must have a MerklePath attached after deserialization")
	assert.Equal(t, uint32(2000), parentEntry.Transaction.MerklePath.BlockHeight,
		"parentTx MerklePath must come from bump1 (block 2000), not bump0 (block 1000)")
}
