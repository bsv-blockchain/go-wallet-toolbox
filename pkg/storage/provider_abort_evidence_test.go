package storage_test

import (
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// AbortAction decides on evidence, not on a status allow-list: an action may be released
// exactly while its shared KnownTx provably never reached a broadcaster. Parking that KnownTx
// is both the decision and the stop signal - no pipeline can claim a parked transaction for a
// post afterwards, so releasing the inputs cannot turn into a double spend.

func TestAbortAction_ReleasesQueuedDelayedActionAndStopsItsBroadcast(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: a delayed action that is queued for broadcast but was never posted
	createResult, signedTx := given.Action(activeStorage).
		WithDelayedBroadcast().
		Created()
	txID := signedTx.TxID().String()

	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true, // parks it as noSend: queued, never handed to a broadcaster
		Reference: to.Ptr(createResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:     signedTx.Bytes(),
	})
	require.NoError(t, err)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusNoSend).WasBroadcast(false)

	// when:
	result, err := activeStorage.AbortAction(t.Context(), testusers.Alice.AuthID(), wdk.AbortActionArgs{
		Reference: primitives.Base64String(createResult.Reference),
	})

	// then:
	require.NoError(t, err)
	require.True(t, result.Aborted)

	// and: the KnownTx is parked, so nothing can claim it for a post anymore
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusAborted)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusInvalid)

	// and: the send_waiting sweep leaves it alone
	_, err = activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)
	require.NoError(t, err)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusInvalid).WasBroadcast(false)

	// and: its inputs are usable again
	thenDBState.CanCreateActionForSatoshis(testusers.Alice, 1)
}

func TestAbortAction_RefusesActionWithBroadcastEvidence(t *testing.T) {
	// given: a processed (broadcast) action
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	createResult, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	// when:
	_, err := activeStorage.AbortAction(t.Context(), testusers.Alice.AuthID(), wdk.AbortActionArgs{
		Reference: primitives.Base64String(createResult.Reference),
	})

	// then:
	require.ErrorIs(t, err, wdk.ErrNotAbortableAction)

	// and: nothing about the transaction changed
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusUnproven)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
}
