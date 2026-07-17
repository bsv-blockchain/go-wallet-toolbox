package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestAbortActionSuccess(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	createResult, _ := given.Action(activeStorage).Created()

	// when:
	result, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: primitives.Base64String(createResult.Reference),
		},
	)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Aborted)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusFailed)
	thenDBState.AllOutputs(testusers.Alice).WithCount(1)
}

func TestAbortActionSuccessfulSpendingAfterAbort(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	const initialTopUp = 100_000

	activeStorage := given.Provider().GORM()
	createResult, _ := given.Action(activeStorage).WithSatoshisToInternalize(initialTopUp).Created()

	// when:
	_, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: primitives.Base64String(createResult.Reference),
		},
	)

	// then:
	require.NoError(t, err)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusFailed)

	// and:
	testabilities.ThenFunds(t, testusers.Alice, activeStorage).
		ShouldBeAbleToReserveSatoshis(initialTopUp)
}

func TestAbortActionErrorCases(t *testing.T) {
	tests := map[string]struct {
		setupTransaction func(given testabilities.StorageFixture) (string, wdk.AuthID)
		expectedErrors   []string
	}{
		"transaction not found by reference": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				return "bm9uLWV4aXN0ZW50LXJlZg==", testusers.Alice.AuthID()
			},
			expectedErrors: []string{
				"failed to abort action",
				"no transaction found with reference or txid",
			},
		},
		"transaction not found by TxID": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				return "1234567890123456789012345678901234567890123456789012345678901234", testusers.Alice.AuthID()
			},
			expectedErrors: []string{
				"failed to abort action",
				"no transaction found with reference or txid",
			},
		},
		"transaction not outgoing - TxID as Reference": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

				return txSpec.ID().String(), testusers.Alice.AuthID()
			},
			expectedErrors: []string{
				wdk.ErrNotAbortableAction.Error(),
				"must be an outgoing transaction",
			},
		},
		"transaction not outgoing - Reference": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

				return fixtures.FaucetReference(txSpec.ID().String()), testusers.Alice.AuthID()
			},
			expectedErrors: []string{
				wdk.ErrNotAbortableAction.Error(),
				"must be an outgoing transaction",
			},
		},
		"different user transaction - Reference": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				createResult, _ := given.Action(activeStorage).Created()

				return createResult.Reference, testusers.Bob.AuthID()
			},
			expectedErrors: []string{
				"failed to abort action:",
				"no transaction found with reference or txid",
			},
		},
		"different user transaction - txID as Reference": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				_, tx := given.Action(activeStorage).Created()

				return tx.TxID().String(), testusers.Bob.AuthID()
			},
			expectedErrors: []string{
				"failed to abort action:",
				"no transaction found with reference or txid",
			},
		},
		"invalid user ID": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				return "some-reference", wdk.AuthID{UserID: nil}
			},
			expectedErrors: []string{
				"access is denied due to an authorization error",
			},
		},
		"transaction with status failed - Reference": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				createResult, _ := given.Action(activeStorage).Created()

				abortResult, err := activeStorage.AbortAction(
					t.Context(),
					testusers.Alice.AuthID(),
					wdk.AbortActionArgs{
						Reference: primitives.Base64String(createResult.Reference),
					},
				)
				require.NoError(t, err)
				require.NotEmpty(t, abortResult)
				require.True(t, abortResult.Aborted)

				return createResult.Reference, testusers.Alice.AuthID()
			},
			expectedErrors: []string{
				wdk.ErrNotAbortableAction.Error(),
				"action with status failed cannot be aborted",
			},
		},
		"transaction with status unproven - Reference": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				createResult, _ := given.Action(activeStorage).Processed()

				return createResult.Reference, testusers.Alice.AuthID()
			},
			expectedErrors: []string{
				"action with status unproven cannot be aborted",
			},
		},
		"transaction with status unproven - TxID as Reference": {
			setupTransaction: func(given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				_, tx := given.Action(activeStorage).Processed()

				return tx.TxID().String(), testusers.Alice.AuthID()
			},
			expectedErrors: []string{
				"action with status unproven cannot be aborted",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			activeStorage := given.Provider().GORM()
			reference, user := test.setupTransaction(given)

			// when:
			_, err := activeStorage.AbortAction(
				t.Context(),
				user,
				wdk.AbortActionArgs{Reference: primitives.Base64String(reference)},
			)

			// then:
			require.Error(t, err)
			for _, expectedError := range test.expectedErrors {
				require.Contains(t, err.Error(), expectedError)
			}
		})
	}
}

func TestAbortActionAbortableStatuses(t *testing.T) {
	tests := map[string]struct {
		setupTransaction func(given testabilities.StorageFixture, activeStorage *storage.Provider) (string, wdk.AuthID)
	}{
		"unsigned_transaction": {
			setupTransaction: func(given testabilities.StorageFixture, activeStorage *storage.Provider) (string, wdk.AuthID) {
				createResult, _ := given.Action(activeStorage).Created()
				return createResult.Reference, testusers.Alice.AuthID()
			},
		},
		"unprocessed_transaction": {
			setupTransaction: func(given testabilities.StorageFixture, activeStorage *storage.Provider) (string, wdk.AuthID) {
				createResult, _ := given.Action(activeStorage).Unprocessed()
				return createResult.Reference, testusers.Alice.AuthID()
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			activeStorage := given.Provider().GORM()
			reference, user := test.setupTransaction(given, activeStorage)

			// when:
			result, err := activeStorage.AbortAction(
				t.Context(),
				user,
				wdk.AbortActionArgs{Reference: primitives.Base64String(reference)},
			)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			require.True(t, result.Aborted)

			thenDBState := testabilities.ThenDBState(t, activeStorage)
			thenDBState.HasUserTransactionByReference(testusers.Alice, reference).
				WithStatus(wdk.TxStatusFailed)
		})
	}
}

func TestProcessAction_AbortUnprocessedTransaction_AndRecreateUTXOs(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	// and:
	const (
		satoshisToInternalize = 5000
		satoshisToSend        = 1000
	)

	// and:
	createActionResult, _ := given.Action(activeStorage).
		WithSatoshisToInternalize(satoshisToInternalize).
		WithSatoshisToSend(satoshisToSend).
		Unprocessed()

	// when:
	abortResult, err := activeStorage.AbortAction(t.Context(), testusers.Alice.AuthID(), wdk.AbortActionArgs{
		Reference: primitives.Base64String(createActionResult.Reference),
	})

	// then:
	require.NoError(t, err)
	require.True(t, abortResult.Aborted)

	// and:
	testabilities.ThenFunds(t, testusers.Alice, activeStorage).
		ShouldBeAbleToReserveSatoshis(satoshisToInternalize)
}

func TestAbortActionRefusedWhenKnownTxInFlight(t *testing.T) {
	// Regression test for P4 (abort is an input-release path): user-initiated
	// AbortAction must refuse when the shared KnownTx shows broadcast/network
	// evidence, even though the local Transaction row is still 'unprocessed'.
	// This reproduces the window between MarkKnownTxsAsSubmitting (KnownTx ->
	// 'sending') and the per-tx status write after broadcast (process.go),
	// where the background sweep (#926) is already gated but user-initiated
	// abort previously was not.

	// given: Alice has a fully processed (broadcast) tx
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	createResult, signedTx := given.Action(activeStorage).Processed()
	txID := signedTx.TxID().String()

	txRows, err := activeStorage.TransactionEntity().Read().
		Reference().Equals(createResult.Reference).
		Find(t.Context())
	require.NoError(t, err)
	require.Len(t, txRows, 1)
	transactionID := txRows[0].ID

	// and: force the DB back into the race window - KnownTx is 'sending'
	// (in-flight broadcast) while the local Transaction row is 'unprocessed'
	db := activeStorage.Database.DB
	require.NoError(t, db.Exec(
		`UPDATE bsv_known_txes SET status = ? WHERE tx_id = ?`,
		string(wdk.ProvenTxStatusSending), txID,
	).Error)
	require.NoError(t, db.Exec(
		`UPDATE bsv_transactions SET status = ? WHERE id = ?`,
		string(wdk.TxStatusUnprocessed), transactionID,
	).Error)

	// when:
	_, err = activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: primitives.Base64String(createResult.Reference),
		},
	)

	// then: abort is refused
	require.Error(t, err)
	require.ErrorIs(t, err, wdk.ErrNotAbortableAction)

	// and: the Transaction row was not touched by the abort path
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusUnprocessed)

	// and: the KnownTx row remains untouched (multi-user design)
	thenDBState.HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusSending)

	// and: the tx's inputs are still claimed (not released back to spendable)
	spentOutputs, err := activeStorage.OutputsEntity().Read().
		SpentBy().Equals(transactionID).
		Find(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, spentOutputs, "expected the aborted tx's inputs to still be claimed (spent_by unchanged)")
}

func TestAbortAction_CreatedOutputsAreMarkedNotSpendable(t *testing.T) {
	// Regression test: outputs created by an aborted TX must be marked spendable=false.
	// Before this fix, they remained spendable=true with a null tx_id, so ListOutputs
	// could return them with a zero outpoint, causing "duplicate input" errors downstream.

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and: an unsigned TX is created (tx_id is null at this point)
	createResult, _ := given.Action(activeStorage).
		WithSatoshisToInternalize(10_000).
		WithSatoshisToSend(1_000).
		Created()

	// and: find the internal transaction row ID to query its outputs
	txRows, err := activeStorage.TransactionEntity().Read().
		Reference().Equals(createResult.Reference).
		Find(t.Context())
	require.NoError(t, err)
	require.Len(t, txRows, 1)
	transactionID := txRows[0].ID

	// when:
	_, err = activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{Reference: primitives.Base64String(createResult.Reference)},
	)
	require.NoError(t, err)

	// then: outputs created by the aborted TX are all not spendable
	outputs, err := activeStorage.OutputsEntity().Read().
		TransactionID().Equals(transactionID).
		Find(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, outputs, "expected to find outputs for the aborted transaction")

	for _, output := range outputs {
		assert.False(t, output.Spendable,
			"output vout=%d from aborted TX must not be spendable", output.Vout)
	}
}
