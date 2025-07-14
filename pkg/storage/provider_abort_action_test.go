package storage_test

import (
	"fmt"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func TestAbortAction_Success(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
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
}

func TestAbortAction_SuccessWithTxID(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, signedTx := given.ActionCreatedAndSigned(activeStorage)
	fmt.Printf("Created transaction with txid: %s\n", signedTx.TxID().String())
	// when:
	result, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: &createResult.Reference,
		},
	)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Aborted)

	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusFailed)
}

func TestAbortAction_InvalidUserID(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.AbortAction(
		t.Context(),
		wdk.AuthID{UserID: nil}, // Invalid auth
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

func TestAbortAction_NotOutgoingTransaction(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	_, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol),
	)
	require.NoError(t, err)

	// when:
	_, err = activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr("internalize-reference"),
		},
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "WERR_INVALID_PARAMETER")
	require.Contains(t, err.Error(), "inprocess, outgoing action")
}

func TestAbortAction_TransactionStatusCompleted(t *testing.T) {
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
	require.Contains(t, err.Error(), "TxStatusInvalid")
	require.Contains(t, err.Error(), "action with status unproven cannot be aborted")
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
	require.Contains(t, err.Error(), "WERR_INVALID_PARAMETER")
}

func TestAbortAction_TransactionStatusSending(t *testing.T) {
	// Note: This test would require a more complex setup to get a transaction
	// into "sending" status, which typically happens through the broadcasting process.
	// For now, we'll test that a processed transaction (unproven) can't be aborted.

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
	require.Contains(t, err.Error(), "WERR_INVALID_PARAMETER")
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

		given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
		createResult, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			fixtures.DefaultValidCreateActionArgs(),
		)
		require.NoError(t, err)

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
	})

	t.Run("signed_transaction", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		activeStorage := given.Provider().GORM()

		given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
		createResult, _ := given.ActionCreatedAndSigned(activeStorage)

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
	})
}

func TestAbortAction_WithProvenTxReq(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, signedTx := given.ActionCreatedAndSigned(activeStorage)

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

	// and: verify transaction status is failed
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusFailed)

	thenDBState.HasKnownTX(signedTx.TxID().String()).
		WithStatus(wdk.ProvenTxStatusInvalid)
}
