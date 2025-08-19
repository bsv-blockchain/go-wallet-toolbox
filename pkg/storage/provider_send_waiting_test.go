package storage_test

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSendWaitingTransactions(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).WithDelayedBroadcast().Created()
	txID := signedTx.TxID().String()

	// and:
	given.Provider().ARC().WhenQueryingTx(txID).WillReturnNoBody()

	// and:
	args := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  to.Ptr(createActionResult.Reference),
		TxID:       to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:      signedTx.Bytes(),
		SendWith:   []primitives.TXIDHexString{},
	}
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)
	require.NoError(t, err)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)

	// when:
	given.Provider().ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()
	err = activeStorage.SendWaitingTransactions(t.Context(), -time.Minute) // NOTE: using negative aged limit to ensure all waiting transactions are sent

	// then:
	require.NoError(t, err)

	// and db state:
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusUnmined)
}
