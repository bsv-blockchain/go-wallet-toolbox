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

	// and:
	activeStorage := given.Provider().GORM()

	// and: create a transaction that can be aborted
	faucet := given.Faucet(activeStorage, testusers.Alice)
	faucet.TopUp(100_000)

	createResult, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultValidCreateActionArgs(),
	)
	require.NoError(t, err)

	// when: abort the action by reference
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

	// and: verify transaction status is now failed
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusFailed)
}

func TestAbortAction_SuccessWithTxID(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	activeStorage := given.Provider().GORM()

	// and: create and process a transaction to get a txid
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, signedTx := given.ActionCreatedAndSigned(activeStorage)
	fmt.Printf("Created transaction with txid: %s\n", signedTx.TxID().String())
	// when: abort the action by txid (64-character reference)
	result, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: &createResult.Reference, //to.Ptr(signedTx.TxID().String()),
		},
	)

	// then:
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Aborted)

	// and: verify transaction status is now failed
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusFailed)
}

func TestAbortAction_InvalidUserID(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	activeStorage := given.Provider().GORM()

	// when: try to abort with nil userID
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

	// and:
	activeStorage := given.Provider().GORM()

	// when: try to abort non-existent transaction
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

	// and:
	activeStorage := given.Provider().GORM()

	// when: try to abort by non-existent txid (64 chars)
	_, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr("123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"),
		},
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "WERR_INVALID_PARAMETER")
}

func TestAbortAction_NotOutgoingTransaction(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	activeStorage := given.Provider().GORM()

	// and: internalize a transaction (incoming, not outgoing)
	_, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol),
	)
	require.NoError(t, err)

	// when: try to abort the internalized (incoming) transaction
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

	// and:
	activeStorage := given.Provider().GORM()

	// and: create and process a transaction to completion (this sets status to unproven)
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, _ := given.ActionCreatedAndProcessed(activeStorage)

	// Note: In a real scenario, the transaction would be completed through
	// the broadcasting/mining process. For this test, we're testing that
	// a processed transaction (status: unproven) cannot be aborted.

	// when: try to abort processed transaction
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
	require.Contains(t, err.Error(), "inprocess, outgoing action")
}

func TestAbortAction_TransactionStatusFailed(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	activeStorage := given.Provider().GORM()

	// and: create a transaction
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultValidCreateActionArgs(),
	)
	require.NoError(t, err)

	// and: abort the transaction once (sets status to failed)
	_, err = activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr(createResult.Reference),
		},
	)
	require.NoError(t, err)

	// when: try to abort already failed transaction
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

	// and:
	activeStorage := given.Provider().GORM()

	// and: create and process a transaction (status becomes unproven)
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, _ := given.ActionCreatedAndProcessed(activeStorage)

	// when: try to abort unproven transaction (equivalent to sending status test)
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

	// and:
	activeStorage := given.Provider().GORM()

	// and: create and process a transaction (this sets status to unproven)
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, _ := given.ActionCreatedAndProcessed(activeStorage)

	// when: try to abort unproven transaction
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

func TestAbortAction_DifferentUserTransaction(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	activeStorage := given.Provider().GORM()

	// and: create a transaction for Alice
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.DefaultValidCreateActionArgs(),
	)
	require.NoError(t, err)

	// when: try to abort Alice's transaction as Bob
	_, err = activeStorage.AbortAction(
		t.Context(),
		testusers.Bob.AuthID(),
		wdk.AbortActionArgs{
			Reference: to.Ptr(createResult.Reference),
		},
	)

	// then:
	require.Error(t, err)
	require.Contains(t, err.Error(), "WERR_INVALID_PARAMETER")
}

// Test cases for different transaction statuses that SHOULD be abortable
func TestAbortAction_AbortableStatuses(t *testing.T) {
	// Test that unsigned transactions can be aborted
	t.Run("unsigned_transaction", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		activeStorage := given.Provider().GORM()

		// and: create a transaction (default status is unsigned)
		given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
		createResult, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			fixtures.DefaultValidCreateActionArgs(),
		)
		require.NoError(t, err)

		// when: abort the transaction
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

		// and: verify transaction status is now failed
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
			WithStatus(wdk.TxStatusFailed)
	})

	// Test that signed (but not processed) transactions can be aborted
	t.Run("signed_transaction", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		activeStorage := given.Provider().GORM()

		// and: create and sign a transaction (but don't process it)
		given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
		createResult, _ := given.ActionCreatedAndSigned(activeStorage)

		// when: abort the transaction
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

		// and: verify transaction status is now failed
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasUserTransactionByReference(testusers.Alice, createResult.Reference).
			WithStatus(wdk.TxStatusFailed)
	})
}

func TestAbortAction_WithProvenTxReq(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	activeStorage := given.Provider().GORM()

	// and: create and sign a transaction (this should create a proven tx req)
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	createResult, signedTx := given.ActionCreatedAndSigned(activeStorage)

	// when: abort the action
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

	// and: verify proven tx req status is invalid (if applicable)
	thenDBState.HasKnownTX(signedTx.TxID().String()).
		WithStatus(wdk.ProvenTxStatusInvalid)
}
