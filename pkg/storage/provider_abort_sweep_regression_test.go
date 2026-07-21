package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TestAbortedTxNotSweptByFailureReview locks that a never-broadcast aborted tx is not
// reclassified to 'failed' by the Monitor failure-review reconciliation
// (reviewKnownTxStatuses / FindKnownTxIDsByStatusesNeedingFailureReview, invoked from
// SynchronizeTransactionStatuses), which only targets KnownTx rows already in
// {invalidTx, doubleSpend}. An aborted-before-broadcast tx has no KnownTx row at all,
// so it must stay 'aborted'.
func TestAbortedTxNotSweptByFailureReview(t *testing.T) {
	// given: an aborted tx (created but never processed/broadcast, so no KnownTx row exists)
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	createResult, _ := given.Action(activeStorage).Created()

	_, err := activeStorage.AbortAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.AbortActionArgs{
			Reference: primitives.Base64String(createResult.Reference),
		},
	)
	require.NoError(t, err)

	testabilities.ThenDBState(t, activeStorage).
		HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusAborted)

	// when: the failure-review reconciliation runs, driven the same way the Monitor
	// runs it in production: via SynchronizeTransactionStatuses.
	_, err = activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	// then: the aborted tx is untouched - still 'aborted', not swept into 'failed'.
	testabilities.ThenDBState(t, activeStorage).
		HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusAborted)
}
