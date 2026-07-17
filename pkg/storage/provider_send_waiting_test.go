package storage_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestSendWaitingTransactions(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	_, signedTx := given.Action(activeStorage).
		WithDelayedBroadcast().
		WillFailOnBroadcast().
		Processed()
	txID := signedTx.TxID().String()

	// and, make sure testabilities are set up correctly:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)

	// when:
	given.Provider().ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()
	_, err := activeStorage.SendWaitingTransactions(t.Context(), -time.Minute) // NOTE: using negative aged limit to ensure all waiting transactions are sent

	// then:
	require.NoError(t, err)

	// and db state:
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
}

func TestSendWaitingTransactions_Empty(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// when:
	_, err := activeStorage.SendWaitingTransactions(t.Context(), -time.Minute) // NOTE: using negative aged limit to ensure all waiting transactions are sent

	// then:
	require.NoError(t, err)
}

func TestSendWaitingTransactions_MinTransactionAge(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	_, signedTx := given.Action(activeStorage).
		WithDelayedBroadcast().
		WillFailOnBroadcast().
		Processed()
	txID := signedTx.TxID().String()

	// and:
	const minTransactionAge = 5 * time.Minute

	// when:
	given.Provider().ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()
	_, err := activeStorage.SendWaitingTransactions(t.Context(), minTransactionAge)

	// then:
	require.NoError(t, err)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending) // The transaction should still be in sending status
}

func TestSendWaitingTransactions_SeveralFailures(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	_, signedTx := given.Action(activeStorage).
		WithDelayedBroadcast().
		WillFailOnBroadcast().
		Processed()
	txID := signedTx.TxID().String()

	// and:
	const tries = 3

	for range tries {
		// when:
		result, err := activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)

		// then: a soft (service-error) failure is NOT a hard error, but the assembled
		// result is now returned (previously SendWaitingTransactions always returned nil, nil).
		require.NoError(t, err)
		require.NotNil(t, result, "SendWaitingTransactions must return the assembled result, not nil")
		require.Len(t, result.NotDelayedResults, 1)
		assert.Equal(t, txID, string(result.NotDelayedResults[0].TxID))
		assert.Equal(t, wdk.ReviewActionResultStatusServiceError, result.NotDelayedResults[0].Status)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)
	}

	// when:
	given.Provider().ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()
	result, err := activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)

	// then: the successful re-post is reflected in the returned result.
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.NotDelayedResults, 1)
	assert.Equal(t, txID, string(result.NotDelayedResults[0].TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusSuccess, result.NotDelayedResults[0].Status)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusUnmined).
		WithAttempts(tries + 2) // +2: one for the initial sending and one for the final successful send
	// NOTE: attempts == tries+2 == number of ACTUAL post attempts (1 initial send + 3 failed
	// re-sends + 1 successful re-send). This guards that attempts track real posts, not queue events.
}

func TestSendWaitingTransactions_ConcurrentCalls(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	_, signedTx := given.Action(activeStorage).
		WithDelayedBroadcast().
		WillFailOnBroadcast().
		Processed()
	txID := signedTx.TxID().String()

	// and:
	const tries = 100
	var wg sync.WaitGroup

	// and:
	given.Provider().ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()
	given.Provider().ARC().HoldBroadcasting() // simulate long blocking broadcasting

	for range tries {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// when:
			_, err := activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)

			// then:
			assert.NoError(t, err)
		}()
	}

	given.Provider().ARC().ReleaseBroadcasting()
	wg.Wait() // wait for all goroutines to finish

	// then db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusUnmined).
		WithAttempts(2) // +1 for the initial sending and +1 for the final successful send

	// NOTE: even though we called SendWaitingTransactions 100 times, the transaction was sent only once
}

// TestSendWaitingTransactions_HardFailureReturnsJoinedError verifies that when a batch hard-fails
// (here: scripts verification errors out before broadcast), SendWaitingTransactions:
//   - returns a non-nil error (previously it swallowed all errors and returned nil, nil), and
//   - keeps processing the remaining batches (continue-on-error), so the joined error references
//     BOTH failing batches rather than bailing out after the first one.
func TestSendWaitingTransactions_HardFailureReturnsJoinedError(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: two independent waiting transactions (distinct amounts => distinct txs => distinct batches).
	_, signedTx1 := given.Action(activeStorage).
		WithDelayedBroadcast().
		WillFailOnBroadcast().
		WithSatoshisToInternalize(5000).
		WithSatoshisToSend(1).
		Processed()
	txID1 := signedTx1.TxID().String()

	_, signedTx2 := given.Action(activeStorage).
		WithDelayedBroadcast().
		WillFailOnBroadcast().
		WithSatoshisToInternalize(6000).
		WithSatoshisToSend(1).
		Processed()
	txID2 := signedTx2.TxID().String()

	// and: both are queued as waiting.
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID1).WithStatus(wdk.ProvenTxStatusSending)
	thenDBState.HasKnownTX(txID2).WithStatus(wdk.ProvenTxStatusSending)

	// and: scripts verification hard-fails for every batch.
	givenProvider.ScriptsVerifier().WillReturnError(fmt.Errorf("mock scripts verifier error"))

	// when:
	result, err := activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)

	// then: a hard failure is surfaced as a non-nil error, and the result is still non-nil.
	require.Error(t, err)
	require.NotNil(t, result)

	// and: both batches were attempted (continue-on-error) - the joined error references both txIDs.
	assert.Contains(t, err.Error(), txID1)
	assert.Contains(t, err.Error(), txID2)

	// and: a hard failure before broadcast must NOT bump attempts (nothing was actually posted);
	// each tx still carries only the single attempt from its initial send.
	thenDBState.HasKnownTX(txID1).WithStatus(wdk.ProvenTxStatusSending).WithAttempts(1)
	thenDBState.HasKnownTX(txID2).WithStatus(wdk.ProvenTxStatusSending).WithAttempts(1)
}

// TestSendWaitingTransactions_DelayedBroadcastCountsOnlyActualPost guards attempts honesty on the
// delayed path: a delayed Process queues the tx WITHOUT bumping attempts, and the single actual
// post (performed by the background broadcaster) bumps it exactly once - so attempts == 1, never
// inflated by a queue-time bump.
func TestSendWaitingTransactions_DelayedBroadcastCountsOnlyActualPost(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a delayed-broadcast action.
	createActionResult, signedTx := given.Action(activeStorage).WithDelayedBroadcast().Created()
	txID := signedTx.TxID().String()

	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  true,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	}

	// when: processed as delayed (queued to the background broadcaster).
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)
	require.NoError(t, err)

	// then: after the background broadcaster actually posts it, attempts reflects exactly one post.
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.WaitForTxStatusByReference(testusers.Alice, createActionResult.Reference, wdk.TxStatusUnproven, 5*time.Second)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusUnmined).
		WithAttempts(1)
}

// TODO: Add test case for batches when noSend..noSend..sendWith scenario is implemented
