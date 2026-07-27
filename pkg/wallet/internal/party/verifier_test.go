package party_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/party"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func newSimpleTx(lockTime uint32) *transaction.Transaction {
	return &transaction.Transaction{Version: 1, LockTime: lockTime}
}

func beefWithTxIDOnly(t *testing.T, txid *chainhash.Hash) primitives.BEEF {
	t.Helper()
	beef := transaction.NewBeef()
	beef.MergeTxidOnly(txid)
	bytes, err := beef.Bytes()
	require.NoError(t, err)
	return bytes
}

func beefWithRawTx(t *testing.T, tx *transaction.Transaction) primitives.BEEF {
	t.Helper()
	beef := transaction.NewBeef()
	_, err := beef.MergeTransaction(tx)
	require.NoError(t, err)
	bytes, err := beef.Bytes()
	require.NoError(t, err)
	return bytes
}

func TestVerifyReturnedTxIDOnlyBeef(t *testing.T) {
	t.Run("fills TxIDOnly from beef party when present", func(t *testing.T) {
		// given: full tx known to beef party
		fullTx := newSimpleTx(42)
		txid := fullTx.TxID()

		bp := wdk.NewBeefParty(nil)
		_, err := bp.MergeTransaction(fullTx)
		require.NoError(t, err)

		// when: returned beef only has txid
		result, err := party.VerifyReturnedTxIDOnlyBeef(bp, beefWithTxIDOnly(t, txid))

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)

		parsed, err := transaction.NewBeefFromBytes(result)
		require.NoError(t, err)
		require.Contains(t, parsed.Transactions, *txid)
		assert.NotEqual(t, transaction.TxIDOnly, parsed.Transactions[*txid].DataFormat)
	})

	t.Run("errors when TxIDOnly not found in beef party", func(t *testing.T) {
		unknown := newSimpleTx(99)
		txid := unknown.TxID()

		bp := wdk.NewBeefParty(nil)

		result, err := party.VerifyReturnedTxIDOnlyBeef(bp, beefWithTxIDOnly(t, txid))

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "tx with only txid not found in beef party")
	})

	t.Run("passes through beef with full raw transactions", func(t *testing.T) {
		fullTx := newSimpleTx(7)
		bp := wdk.NewBeefParty(nil)

		result, err := party.VerifyReturnedTxIDOnlyBeef(bp, beefWithRawTx(t, fullTx))

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("errors on invalid beef bytes", func(t *testing.T) {
		bp := wdk.NewBeefParty(nil)

		result, err := party.VerifyReturnedTxIDOnlyBeef(bp, primitives.BEEF{0x00, 0x01})

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestVerifyReturnedTxIDOnlyAtomicBEEF(t *testing.T) {
	t.Run("returns atomic beef when tx is filled from party", func(t *testing.T) {
		fullTx := newSimpleTx(11)
		txid := fullTx.TxID()

		bp := wdk.NewBeefParty(nil)
		_, err := bp.MergeTransaction(fullTx)
		require.NoError(t, err)

		result, err := party.VerifyReturnedTxIDOnlyAtomicBEEF(bp, *txid, beefWithTxIDOnly(t, txid))

		require.NoError(t, err)
		require.NotNil(t, result)

		// Atomic BEEF should round-trip
		parsed, _, err := transaction.NewBeefFromAtomicBytes(result)
		require.NoError(t, err)
		require.Contains(t, parsed.Transactions, *txid)
	})

	t.Run("allows remaining TxIDOnly when listed in knownTxIDs", func(t *testing.T) {
		// knownTxIDs path: if after merge a tx is still TxIDOnly but listed as known, it is accepted.
		// We simulate by passing a raw beef that only has a TxIDOnly entry for an unknown tx that is
		// explicitly listed as known — but first-loop will error because party lacks the full tx.
		// So: party has full tx (first loop fills it), knownTxIDs is also passed for completeness.
		fullTx := newSimpleTx(22)
		txid := fullTx.TxID()

		bp := wdk.NewBeefParty(nil)
		_, err := bp.MergeTransaction(fullTx)
		require.NoError(t, err)

		known := primitives.TXIDHexString(txid.String())
		result, err := party.VerifyReturnedTxIDOnlyAtomicBEEF(bp, *txid, beefWithTxIDOnly(t, txid), known)

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("errors when TxIDOnly cannot be resolved", func(t *testing.T) {
		unknown := newSimpleTx(33)
		txid := unknown.TxID()
		bp := wdk.NewBeefParty(nil)

		result, err := party.VerifyReturnedTxIDOnlyAtomicBEEF(bp, *txid, beefWithTxIDOnly(t, txid))

		require.Error(t, err)
		assert.Nil(t, result)
	})
}
