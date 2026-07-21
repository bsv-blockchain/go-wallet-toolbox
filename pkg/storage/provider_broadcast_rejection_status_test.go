package storage_test

import (
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TestBroadcastRejectionStaysFailed locks the #959 divergence: a transaction that
// reached the broadcast endpoint and was rejected (double spend) must land on
// TxStatusFailed, never TxStatusAborted. Only pre-broadcast aborts use 'aborted'.
func TestBroadcastRejectionStaysFailed(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	provider := given.Provider()
	activeStorage := provider.WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	createResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()
	competingTxID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(txID).WillReturnDoubleSpending(competingTxID)

	// when: the tx is broadcast and rejected as a double spend
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		Reference: to.Ptr(createResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:     signedTx.Bytes(),
		SendWith:  []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// then: it is 'failed' (permanent), NOT 'aborted' (retryable)
	testabilities.ThenDBState(t, activeStorage).
		HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusFailed)
}
