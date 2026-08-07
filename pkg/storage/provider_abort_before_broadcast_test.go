package storage_test

import (
	"fmt"
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

// A pre-broadcast abort releases the inputs of a transaction, so the shared KnownTx must be
// parked in the same breath. Leaving it in a broadcastable status (delayed processing sets
// 'unsent', which send_waiting selects) would let the retry post a transaction whose inputs
// are spendable again - a real double spend.
func TestProcessAction_DelayedPreBroadcastAbort_ParksKnownTxSoSendWaitingSkipsIt(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).
		WithDelayedBroadcast().
		Created()
	txID := signedTx.TxID().String()

	// and: script verification fails, which happens before anything is queued or posted
	given.Provider().ScriptsVerifier().WillReturnError(fmt.Errorf("mock scripts verifier error"))

	args := wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsDelayed: true,
		Reference: to.Ptr(createActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:     signedTx.Bytes(),
		SendWith:  []primitives.TXIDHexString{},
	}

	// when:
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.Error(t, err)

	// and: the action is released and the shared KnownTx is no longer broadcastable
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResult.Reference).
		WithStatus(wdk.TxStatusAborted)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusInvalid).
		WithAttempts(0).
		WasBroadcast(false)

	// when: the send_waiting sweep runs
	_, err = activeStorage.SendWaitingTransactions(t.Context(), -time.Minute)

	// then: it must not pick the aborted transaction up
	require.NoError(t, err)
	thenDBState.HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusInvalid).
		WithAttempts(0).
		WasBroadcast(false)
}
