package wdk_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// A wallet advertises the txids its beef party holds, and storage answers with
// bare txids for exactly those. Anything the party drops in between is a
// transaction the wallet claimed to have and then cannot produce, which fails
// the whole call.
//
// Bounding the graph is therefore only safe where it cannot strand a response
// already in flight.

func boundTestTx(lockTime uint32) *transaction.Transaction {
	tx := &transaction.Transaction{Version: 1, LockTime: lockTime}
	tx.Outputs = append(tx.Outputs, &transaction.TransactionOutput{
		Satoshis:      1000,
		LockingScript: &script.Script{},
	})
	return tx
}

func TestBeefPartyKeepsAdvertisedTxsResolvable(t *testing.T) {
	const storageParty = "storage"
	// Enough rounds to carry the graph past its cap, which is where Alice's
	// wallet broke: 257 transactions against a 256 cap.
	const (
		rounds       = 6
		txsPerAnswer = 60
	)

	bp := wdk.NewBeefParty([]string{storageParty})
	lockTime := uint32(0)

	for round := 1; round <= rounds; round++ {
		// What the wallet tells storage it already holds. Reading the graph is
		// also where it gets bounded, so this may legitimately come back empty
		// right after a prune.
		advertised := bp.ValidateTransactions(t.Context()).Valid

		// Storage answers with bare txids for everything advertised, plus a
		// batch of transactions the wallet has not seen before.
		answer := transaction.NewBeef()
		for _, txid := range advertised {
			hash, err := chainhash.NewHashFromHex(txid)
			require.NoError(t, err)
			answer.MergeTxidOnly(hash)
		}
		for range txsPerAnswer {
			lockTime++
			_, err := answer.MergeTransaction(boundTestTx(lockTime))
			require.NoError(t, err)
		}

		answerBytes, err := answer.Bytes()
		require.NoError(t, err)
		require.NoError(t, bp.MergeBeefFromParty(t.Context(), storageParty, primitives.BEEF(answerBytes)))

		// Everything advertised this round must still be resolvable, or the
		// wallet cannot produce transactions it just claimed to hold - which is
		// exactly the "tx with only txid not found in beef party" failure.
		err = bp.WithLock(t.Context(), func(graph *transaction.Beef) error {
			for _, txid := range advertised {
				assert.NotNilf(t, graph.FindAtomicTransaction(txid),
					"round %d: advertised tx %s is no longer resolvable", round, txid)
			}
			return nil
		})
		require.NoError(t, err)
	}

	// The bound still has to do its job, or this only passes by never pruning.
	err := bp.WithLock(t.Context(), func(graph *transaction.Beef) error {
		assert.LessOrEqualf(t, len(graph.Transactions), wdk.DefaultMaxGraphTxs+txsPerAnswer,
			"graph grew unbounded: %d transactions", len(graph.Transactions))
		return nil
	})
	require.NoError(t, err)
}

// The same story with two actions overlapping, which is the shape that broke
// under load: one action advertises, a second one's merge pushes the graph past
// its cap, and the reset drops the graph the first action still needs to resolve
// the reply it is holding. A lease defers the reset until the window closes.
func TestBeefPartyDefersResetWhileAnActionIsBetweenAdvertiseAndResolve(t *testing.T) {
	const storageParty = "storage"

	bp := wdk.NewBeefParty([]string{storageParty})
	lockTime := uint32(0)

	mergeBatch := func(count int) {
		answer := transaction.NewBeef()
		for range count {
			lockTime++
			_, err := answer.MergeTransaction(boundTestTx(lockTime))
			require.NoError(t, err)
		}
		answerBytes, err := answer.Bytes()
		require.NoError(t, err)
		require.NoError(t, bp.MergeBeefFromParty(t.Context(), storageParty, primitives.BEEF(answerBytes)))
	}

	// Action A opens its window and advertises what the graph holds.
	mergeBatch(10)
	releaseA := bp.Lease(t.Context())
	advertised := bp.ValidateTransactions(t.Context()).Valid
	require.NotEmpty(t, advertised)

	// Action B, concurrently, pushes the graph past its cap and reads it - the
	// point at which the old code reset the graph out from under A.
	mergeBatch(wdk.DefaultMaxGraphTxs + 1)
	releaseB := bp.Lease(t.Context())
	bp.ValidateTransactions(t.Context())
	bp.PruneIfOversized(t.Context())
	releaseB()

	err := bp.WithLock(t.Context(), func(graph *transaction.Beef) error {
		for _, txid := range advertised {
			assert.NotNilf(t, graph.FindAtomicTransaction(txid),
				"advertised tx %s must stay resolvable while its action's window is open", txid)
		}
		return nil
	})
	require.NoError(t, err)

	// Once A closes its window the deferred reset lands, so the bound still holds.
	releaseA()

	err = bp.WithLock(t.Context(), func(graph *transaction.Beef) error {
		assert.Empty(t, graph.Transactions, "the deferred reset must run when the last lease is released")
		return nil
	})
	require.NoError(t, err)
}

// A lease must not be able to hold the graph open forever: past a hard ceiling
// the reset happens anyway, so sustained concurrency cannot reintroduce
// unbounded growth.
func TestBeefPartyResetsPastTheCeilingEvenWithAnOpenLease(t *testing.T) {
	const storageParty = "storage"

	bp := wdk.NewBeefParty([]string{storageParty})
	lockTime := uint32(0)

	release := bp.Lease(t.Context())
	defer release()

	answer := transaction.NewBeef()
	for range wdk.DefaultMaxGraphTxs*wdk.EmergencyResetFactor + 1 {
		lockTime++
		_, err := answer.MergeTransaction(boundTestTx(lockTime))
		require.NoError(t, err)
	}
	answerBytes, err := answer.Bytes()
	require.NoError(t, err)
	require.NoError(t, bp.MergeBeefFromParty(t.Context(), storageParty, primitives.BEEF(answerBytes)))

	bp.PruneIfOversized(t.Context())

	err = bp.WithLock(t.Context(), func(graph *transaction.Beef) error {
		assert.Empty(t, graph.Transactions, "the ceiling must reset the graph despite the open lease")
		return nil
	})
	require.NoError(t, err)
}
