package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

func TestAbortActionSuccess(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	createResult, _ := given.ActionCreatedAndSigned(activeStorage)

	// when:
	result, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: createResult.Reference,
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

	activeStorage := given.Provider().GORM()
	createResult, _ := given.ActionCreatedAndSigned(activeStorage)

	// when:
	result, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: createResult.Reference,
		},
	)
	require.NoError(t, err)

	createResult, err = activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultValidCreateActionArgs(),
	)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Aborted)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusUnsigned)
}

func TestAbortActionErrorCases(t *testing.T) {
	tests := map[string]struct {
		setupTransaction func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID)
		args             func(reference string) wdk.AbortActionArgs
		expectedErrors   []string
	}{
		"transaction not found by reference": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				return "non-existent-reference", testusers.Alice.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
			expectedErrors: []string{
				"failed to abort action",
				"failed to find unique transaction by reference",
				"non-existent-reference",
			},
		},
		"transaction not found by TxID": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				return "1234567890123456789012345678901234567890123456789012345678901234", testusers.Alice.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
			expectedErrors: []string{"found 0"},
		},
		"transaction not outgoing - TxID as Reference": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

				return txSpec.ID().String(), testusers.Alice.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
			expectedErrors: []string{
				wdk.ErrNotAbortableAction.Error(),
				"must be an outgoing transaction"},
		},
		"transaction not outgoing - Reference": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

				return fixtures.FaucetReference(txSpec.ID().String()), testusers.Alice.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
			expectedErrors: []string{
				wdk.ErrNotAbortableAction.Error(),
				"must be an outgoing transaction"},
		},
		"different user transaction - Reference": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				createResult, _ := given.ActionCreatedAndSigned(activeStorage)

				return createResult.Reference, testusers.Bob.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
			expectedErrors: []string{
				"expected exactly one transaction with reference",
				"found 0",
			},
		},
		"different user transaction - txID as Reference": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				_, tx := given.ActionCreatedAndSigned(activeStorage)

				return tx.TxID().String(), testusers.Bob.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
			expectedErrors: []string{
				"expected exactly one transaction with reference",
				"found 0",
			},
		},
		"invalid user ID": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				return "some-reference", wdk.AuthID{UserID: nil}
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
			expectedErrors: []string{
				"access is denied due to an authorization error",
			},
		},
		"transaction with status failed - Reference": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				createResult, _ := given.ActionCreatedAndSigned(activeStorage)

				abortResult, err := activeStorage.AbortAction(
					t.Context(),
					testusers.Alice.AuthID(),
					wdk.AbortActionArgs{
						Reference: createResult.Reference,
					},
				)
				require.NoError(t, err)
				require.NotEmpty(t, abortResult)
				require.Equal(t, true, abortResult.Aborted)

				return createResult.Reference, testusers.Alice.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
			expectedErrors: []string{
				wdk.ErrNotAbortableAction.Error(),
				"action with status failed cannot be aborted"},
		},
		"transaction with status unproven - Reference": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				createResult, _ := given.ActionCreatedAndProcessed(activeStorage)

				return createResult.Reference, testusers.Alice.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
			expectedErrors: []string{
				"action with status unproven cannot be aborted",
			},
		},
		"transaction with status unproven - TxID as Reference": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				_, tx := given.ActionCreatedAndProcessed(activeStorage)

				return tx.TxID().String(), testusers.Alice.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
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
			reference, user := test.setupTransaction(t, given)

			// when:
			_, err := activeStorage.AbortAction(
				t.Context(),
				user,
				test.args(reference),
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
		setupTransaction func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID)
		args             func(reference string) wdk.AbortActionArgs
	}{
		"unsigned_transaction": {
			setupTransaction: func(t *testing.T, given testabilities.StorageFixture) (string, wdk.AuthID) {
				activeStorage := given.Provider().GORM()
				createResult, _ := given.ActionCreatedAndSigned(activeStorage)
				return createResult.Reference, testusers.Alice.AuthID()
			},
			args: func(reference string) wdk.AbortActionArgs {
				return wdk.AbortActionArgs{Reference: reference}
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			activeStorage := given.Provider().GORM()
			reference, user := test.setupTransaction(t, given)

			// when:
			result, err := activeStorage.AbortAction(
				t.Context(),
				user,
				test.args(reference),
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
