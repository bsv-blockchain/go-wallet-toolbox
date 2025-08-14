package storage_test

import (
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessActionHappyPath(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()

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

	// when:
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusUnproven, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 1)
	reviewActionResult := result.NotDelayedResults[0]
	assert.Equal(t, txID, string(reviewActionResult.TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusSuccess, reviewActionResult.Status)
	assert.Empty(t, reviewActionResult.CompetingTxs)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		HasRawTx().
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.
				Count(5).
				Note("processAction", to.Ptr(testusers.Alice.ID), nil).
				Note("postBeefSuccess", nil, map[string]any{
					"name": "ARC",
				}).
				Note("postBeefError", nil, map[string]any{
					"name": "WhatsOnChain",
				}).
				Note("postBeefError", nil, map[string]any{
					"name": "Bitails",
				}).
				Note("aggregateResults", nil, map[string]any{
					"aggStatus":         "success",
					"doubleSpendCount":  0,
					"serviceErrorCount": 2,
					"statusErrorCount":  0,
					"status_now":        "unmined",
					"successCount":      1,
				})
		})

	thenDBState.HasUserTransactionByReference(testusers.Alice, *args.Reference).
		WithTxID(txID).WithStatus(wdk.TxStatusUnproven)
}

func TestProcessActionTwice(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()

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

	// when:
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	// when:
	args.IsNewTx = false
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusUnproven, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 0)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		HasRawTx()

	thenDBState.HasUserTransactionByReference(testusers.Alice, *args.Reference).
		WithTxID(txID).WithStatus(wdk.TxStatusUnproven)
}

func TestProcessAction_DelayedBroadcast(t *testing.T) {
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

	// when:
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusSending, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 0)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.WaitForTxStatusByReference(testusers.Alice, *args.Reference, wdk.TxStatusUnproven, 2*time.Second)

	// and db state:
	thenDBState.HasKnownTX(txID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		HasRawTx().
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.
				Count(5).
				Note("processAction", to.Ptr(testusers.Alice.ID), nil).
				Note("postBeefSuccess", nil, map[string]any{
					"name": "ARC",
				}).
				Note("postBeefError", nil, map[string]any{
					"name": "WhatsOnChain",
				}).
				Note("postBeefError", nil, map[string]any{
					"name": "Bitails",
				}).
				Note("aggregateResults", nil, map[string]any{
					"aggStatus":         "success",
					"doubleSpendCount":  0,
					"serviceErrorCount": 2,
					"statusErrorCount":  0,
					"status_now":        "unmined",
					"successCount":      1,
				})
		})

	thenDBState.HasUserTransactionByReference(testusers.Alice, *args.Reference).
		WithTxID(txID).WithStatus(wdk.TxStatusUnproven)
}

func TestProcessAction_DelayedBroadcastForManyTransactions(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	const count = 100
	createActionResults := make([]*wdk.StorageCreateActionResult, count)
	signedTxs := make([]*transaction.Transaction, count)
	for i := 0; i < count; i++ {
		satoshisToInternalize := uint64(1000 + i) // this makes transactions different
		createActionResults[i], signedTxs[i] = given.Action(activeStorage).
			WithDelayedBroadcast().
			WithSatoshisToInternalize(satoshisToInternalize).
			WithSatoshisToSend(1).
			Created()
	}

	processActionArgs := make([]wdk.ProcessActionArgs, count)
	for i := 0; i < count; i++ {
		txID := signedTxs[i].TxID().String()
		processActionArgs[i] = wdk.ProcessActionArgs{
			IsNewTx:    true,
			IsSendWith: false,
			IsNoSend:   false,
			IsDelayed:  true,
			Reference:  to.Ptr(createActionResults[i].Reference),
			TxID:       to.Ptr(primitives.TXIDHexString(txID)),
			RawTx:      signedTxs[i].Bytes(),
			SendWith:   []primitives.TXIDHexString{},
		}
	}

	// when:
	var err error
	results := make([]*wdk.ProcessActionResult, count)
	for i := 0; i < count; i++ {
		results[i], err = activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), processActionArgs[i])
		require.NoError(t, err)
	}

	// then:
	for i := 0; i < count; i++ {
		txID := signedTxs[i].TxID().String()
		require.Len(t, results[i].SendWithResults, 1)
		sendWithResult := results[i].SendWithResults[0]
		assert.Equal(t, txID, string(sendWithResult.TxID))
		assert.Equal(t, wdk.SendWithResultStatusSending, sendWithResult.Status)

		require.Len(t, results[i].NotDelayedResults, 0)
	}

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	for i := 0; i < count; i++ {
		txID := signedTxs[i].TxID().String()

		thenDBState.WaitForTxStatusByReference(testusers.Alice, createActionResults[i].Reference, wdk.TxStatusUnproven, 2*time.Second)

		thenDBState.HasKnownTX(txID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined).
			HasRawTx()

		thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResults[i].Reference).
			WithTxID(txID).WithStatus(wdk.TxStatusUnproven)
	}
}

func TestProcessActionErrorCases(t *testing.T) {
	tests := map[string]struct {
		argsModifier func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs
	}{
		"IsNewTx set to false for not stored tx": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.IsNewTx = false
				return args
			},
		},
		"not existing reference": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.Reference = to.Ptr("not-existing-reference")
				return args
			},
		},
		"tx id missmatch": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				otherID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
				args.TxID = to.Ptr(primitives.TXIDHexString(otherID))
				return args
			},
		},
		"empty raw tx": {
			argsModifier: func(args wdk.ProcessActionArgs) wdk.ProcessActionArgs {
				args.RawTx = []byte{}
				return args
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()
			activeStorage := given.Provider().
				WithRandomizer(randomizer.NewTestRandomizer()).
				GORM()

			// and:
			createActionResult, signedTx := given.Action(activeStorage).Created()
			txID := signedTx.TxID().String()

			// and:
			args := test.argsModifier(wdk.ProcessActionArgs{
				IsNewTx:    false,
				IsSendWith: false,
				IsNoSend:   false,
				IsDelayed:  false,
				Reference:  to.Ptr(createActionResult.Reference),
				TxID:       to.Ptr(primitives.TXIDHexString(txID)),
				RawTx:      signedTx.Bytes(),
				SendWith:   []primitives.TXIDHexString{},
			})

			// when:
			_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

			// then:
			require.Error(t, err)
		})
	}
}

func TestProcessActionDoubleSpending(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()

	// and:
	otherTXID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnDoubleSpending(otherTXID)

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

	// when:
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusFailed, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 1)
	reviewActionResult := result.NotDelayedResults[0]
	assert.Equal(t, txID, string(reviewActionResult.TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusDoubleSpend, reviewActionResult.Status)

	require.Len(t, reviewActionResult.CompetingTxs, 1)
	assert.Equal(t, otherTXID, reviewActionResult.CompetingTxs[0])
}

func TestProcessActionARCReturnNoBody(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	givenProvider := given.Provider()
	activeStorage := givenProvider.
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()

	// and:
	givenProvider.ARC().WhenQueryingTx(txID).WillReturnNoBody()

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

	// when:
	result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)

	require.Len(t, result.SendWithResults, 1)
	sendWithResult := result.SendWithResults[0]
	assert.Equal(t, txID, string(sendWithResult.TxID))
	assert.Equal(t, wdk.SendWithResultStatusSending, sendWithResult.Status)

	require.Len(t, result.NotDelayedResults, 1)
	reviewActionResult := result.NotDelayedResults[0]
	assert.Equal(t, txID, string(reviewActionResult.TxID))
	assert.Equal(t, wdk.ReviewActionResultStatusServiceError, reviewActionResult.Status)
	assert.Empty(t, reviewActionResult.CompetingTxs)
}
