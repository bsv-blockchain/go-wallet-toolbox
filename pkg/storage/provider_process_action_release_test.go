package storage_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// The transactions in these tests never make it past SpendTransaction, so they stay
// 'unsigned' and keep their inputs reserved. ProcessAction compensates by aborting them.

func TestProcessAction_ReleasesInputsWhenTxIDDoesNotMatchRawTx(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()

	// and: a txID that does not belong to the provided raw tx
	otherTxID := primitives.TXIDHexString("0000000000000000000000000000000000000000000000000000000000000001")

	args := wdk.ProcessActionArgs{
		IsNewTx:   true,
		Reference: to.Ptr(createActionResult.Reference),
		TxID:      to.Ptr(otherTxID),
		RawTx:     signedTx.Bytes(),
		SendWith:  []primitives.TXIDHexString{},
	}

	// when:
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.ErrorContains(t, err, "txID mismatch")

	// and: the action was released instead of lingering as 'unsigned'
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResult.Reference).
		WithStatus(wdk.TxStatusAborted).
		WithoutTxID()
}

func TestProcessAction_ReleasesInputsOnInvalidArgs(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, _ := given.Action(activeStorage).Created()

	// and: args rejected by validation before Process is even called
	args := wdk.ProcessActionArgs{
		IsNewTx:   true,
		Reference: to.Ptr(createActionResult.Reference),
		TxID:      nil,
		RawTx:     nil,
		SendWith:  []primitives.TXIDHexString{},
	}

	// when:
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.ErrorContains(t, err, "invalid processAction args")

	// and:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResult.Reference).
		WithStatus(wdk.TxStatusAborted)
}

func TestProcessAction_ReleasedInputsCanFundAnotherAction(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()

	args := wdk.ProcessActionArgs{
		IsNewTx:   true,
		Reference: to.Ptr(createActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString("0000000000000000000000000000000000000000000000000000000000000001")),
		RawTx:     signedTx.Bytes(),
		SendWith:  []primitives.TXIDHexString{},
	}

	// when:
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.Error(t, err)

	// and: the funding is usable again right away, without waiting for fail_abandoned
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.CanCreateActionForSatoshis(testusers.Alice, 1)
}

func TestProcessAction_DoesNotReleaseUnknownReference(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and: an unrelated action that must not be touched
	createActionResult, signedTx := given.Action(activeStorage).Created()

	args := wdk.ProcessActionArgs{
		IsNewTx:   true,
		Reference: to.Ptr("reference-of-no-existing-action"),
		TxID:      to.Ptr(primitives.TXIDHexString(signedTx.TxID().String())),
		RawTx:     signedTx.Bytes(),
		SendWith:  []primitives.TXIDHexString{},
	}

	// when:
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.ErrorContains(t, err, "not found in the database")

	// and: the existing action is untouched
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResult.Reference).
		WithStatus(wdk.TxStatusUnsigned)
}

func TestProcessAction_DoesNotReleaseAlreadyProcessedAction(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	createActionResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()

	args := wdk.ProcessActionArgs{
		IsNewTx:   true,
		Reference: to.Ptr(createActionResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:     signedTx.Bytes(),
		SendWith:  []primitives.TXIDHexString{},
	}

	// and: the action is processed successfully
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)
	require.NoError(t, err)

	// when: the same action is submitted again as a new transaction
	_, err = activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.Error(t, err)

	// and: the processed transaction must not be released - it can already be on the network
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createActionResult.Reference).
		WithTxID(txID).
		WithStatus(wdk.TxStatusUnproven)
}
