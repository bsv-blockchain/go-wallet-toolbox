package storage_test

import (
	"fmt"
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities/nosendtest"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// A failing broadcast attempt may only release the transaction the call itself introduced.
// The sendWith companions were parked by their own createAction (noSend) and stay the
// caller's to send later - releasing their inputs here would silently destroy them.
func TestProcessAction_PreBroadcastRelease_LeavesSendWithCompanionsAlone(t *testing.T) {
	// given:
	const inputSatoshis = 99904

	given, when, _, cleanup := nosendtest.New(t, testusers.Alice)
	defer cleanup()

	given.UserOwnsMultipleUTXOsToSpend(inputSatoshis)

	// and: a parked noSend action
	_, _ = when.CreateAndProcessNoSendAction(nil)
	noSendTxIDs := when.NoSendTxs()
	require.Len(t, noSendTxIDs, 1)
	noSendTxID := noSendTxIDs[0]

	// and: a new action that wants to be sent together with it
	createActionResult, signedTx := when.CreateAction(
		fixtures.DefaultValidCreateActionArgs(when.CreateActionSendWithArgsModifier(when.NoSendTxsHexStrings()...)),
	)
	newTxID := signedTx.TxID().String()

	// and: the broadcast attempt fails before anything is posted
	given.Provider().ScriptsVerifier().WillReturnError(fmt.Errorf("mock scripts verifier error"))

	// when:
	_, err := given.ActiveProvider().ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: true,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(newTxID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   when.NoSendTxsHexStrings(),
	})

	// then:
	require.Error(t, err)

	thenDBState := testabilities.ThenDBState(t, given.ActiveProvider())

	// and: the transaction introduced by this call is released
	thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResult.Reference).
		WithStatus(wdk.TxStatusAborted)
	thenDBState.HasKnownTX(newTxID).WithStatus(wdk.ProvenTxStatusInvalid)

	// and: the parked companion is untouched and still sendable
	thenDBState.HasUserTransactionByTxID(testusers.Alice, noSendTxID).
		WithStatus(wdk.TxStatusNoSend)
	thenDBState.HasKnownTX(noSendTxID).WithStatus(wdk.ProvenTxStatusNoSend)
}
