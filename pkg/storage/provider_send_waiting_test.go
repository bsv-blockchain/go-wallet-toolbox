package storage_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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
		_, err := activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)

		// then:
		require.NoError(t, err)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)
	}

	// when:
	given.Provider().ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()
	_, err := activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)

	// then:
	require.NoError(t, err)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusUnmined).
		WithAttempts(tries + 2) // +2: one for the initial sending and one for the final successful send
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

// TODO: Add test case for batches when noSend..noSend..sendWith scenario is implemented
