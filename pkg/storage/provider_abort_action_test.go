package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

const NumberOfDesiredUTXOs = 32

func TestAbortAction_Success(t *testing.T) {
	// given:
	given, cleanup := testabilities.GivenCustomStorage(t, fixtures.StorageServerPrivKey, "dbstorage_test")
	defer cleanup()

	activeStorage := given.Provider().GORM()
	faucet := given.Faucet(activeStorage, testusers.Alice)
	faucet.TopUp(100_000)

	createResult, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultValidCreateActionArgs(),
	)
	require.NoError(t, err)
	testabilities.ThenDBState(t, activeStorage).AllOutputs(testusers.Alice).WithCount(1 + NumberOfDesiredUTXOs)
	// when:
	result, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr(createResult.Reference),
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

func TestAbortAction_TransactionNotOutgoing(t *testing.T) {
	// given:
	given, cleanup := testabilities.GivenCustomStorage(t, fixtures.StorageServerPrivKey, "dbstorage_test")

	defer cleanup()

	activeStorage := given.Provider().GORM()

	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createdTx, _ := given.ActionCreatedAndSigned(activeStorage)
	txID := createdTx.Inputs[0].SourceTxID
	testabilities.ThenDBState(t, activeStorage).AllOutputs(testusers.Alice).WithCount(1 + NumberOfDesiredUTXOs)
	// when:
	result, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: &txID,
		},
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to abort action: txStatusInvalid: must be an outgoing transaction.")
	require.Nil(t, result)
	testabilities.ThenDBState(t, activeStorage).AllOutputs(testusers.Alice).WithCount(1 + NumberOfDesiredUTXOs)
}

func TestAbortAction_InvalidUserID(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.AbortAction(
		t.Context(),
		wdk.AuthID{UserID: nil},
		fixtures.DefaultValidAbortActionArgs(),
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "access is denied due to an authorization error")
}

func TestAbortAction_TransactionNotFound(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr("non-existent-reference"),
		},
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to abort action")
	require.Contains(t, err.Error(), "failed to find unique transaction by reference")
	require.Contains(t, err.Error(), "non-existent-reference")
}

func TestAbortAction_TransactionNotFoundByTxID(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr("123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"),
		},
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "found 0")
}

func TestAbortAction_TransactionStatusFailed(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultValidCreateActionArgs(),
	)
	require.NoError(t, err)

	_, err = activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr(createResult.Reference),
		},
	)
	require.NoError(t, err)

	// when:
	_, err = activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr(createResult.Reference),
		},
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to abort action")
	require.Contains(t, err.Error(), "action with status failed cannot be aborted")
}

func TestAbortAction_TransactionStatusUnproven(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, _ := given.ActionCreatedAndProcessed(activeStorage)

	// when:
	_, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr(createResult.Reference),
		},
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "action with status unproven cannot be aborted")
}

func TestAbortAction_DifferentUserTransaction(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultValidCreateActionArgs(),
	)
	require.NoError(t, err)

	// when:
	_, err = activeStorage.AbortAction(
		t.Context(),
		testusers.Bob.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr(createResult.Reference),
		},
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected exactly one transaction with reference")
	require.Contains(t, err.Error(), "found 0")
}

func TestAbortAction_AbortableStatuses(t *testing.T) {
	t.Run("unsigned_transaction", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().GORM()
		thenDBState := testabilities.ThenDBState(t, activeStorage)

		given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
		createResult, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			fixtures.DefaultValidCreateActionArgs(),
		)
		require.NoError(t, err)
		thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
			WithStatus(wdk.TxStatusUnsigned)

		// when:
		result, err := activeStorage.AbortAction(
			t.Context(),
			testusers.Alice.AuthID(),
			wdk.AbortActionArgs{
				Reference: to.Ptr(createResult.Reference),
			},
		)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.Aborted)

		thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
			WithStatus(wdk.TxStatusFailed)
	})
}
