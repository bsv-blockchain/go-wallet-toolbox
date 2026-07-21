package storage_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestListTransactions_HappyPath(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// and:
	given.Action(activeStorage).Processed()

	// When:
	args := wdk.ListTransactionsArgs{
		Limit:  10,
		Offset: 0,
	}
	result, err := activeStorage.ListTransactions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, int(result.TotalTransactions), 1) //nolint:gosec // test assertion, TotalTransactions fits in int
}

func TestListTransactions_InvalidAuth(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	args := wdk.ListTransactionsArgs{
		Limit:  10,
		Offset: 0,
	}

	// When:
	_, err := activeStorage.ListTransactions(ctx, wdk.AuthID{UserID: nil}, args)

	// Then:
	require.ErrorIs(t, err, storage.ErrAuthorization)
}

func TestListTransactions_EmptyResult(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// When:
	args := wdk.ListTransactionsArgs{
		Limit:  10,
		Offset: 0,
	}
	result, err := activeStorage.ListTransactions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, primitives.PositiveInteger(0), result.TotalTransactions)
	assert.Empty(t, result.Transactions)
}

func TestListTransactions_FilterByTxID(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// Create actions
	_, ownedTx := given.Action(activeStorage).Processed()
	given.Action(activeStorage).WithSatoshisToInternalize(50000).Processed()

	txID := ownedTx.TxID().String()

	// When: filter by specific txID
	args := wdk.ListTransactionsArgs{
		Limit:  10,
		Offset: 0,
		TxIDs:  []string{txID},
	}
	result, err := activeStorage.ListTransactions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, primitives.PositiveInteger(1), result.TotalTransactions)
	assert.Len(t, result.Transactions, 1)
	assert.Equal(t, txID, result.Transactions[0].TxID)
}

func TestListTransactions_FilterByLabels(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// Create actions with labels
	label1 := "label-1"
	label2 := "label-2"
	given.Action(activeStorage).WithLabels(label1).WithSatoshisToInternalize(100001).Processed()
	given.Action(activeStorage).WithLabels(label1, label2).WithSatoshisToInternalize(100002).Processed()
	given.Action(activeStorage).WithSatoshisToInternalize(100003).Processed() // No labels

	// When: filter by label1 (ANY)
	args := wdk.ListTransactionsArgs{
		Limit:  10,
		Offset: 0,
		Labels: []string{label1},
	}
	result, err := activeStorage.ListTransactions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, int(result.TotalTransactions), 2) //nolint:gosec // test assertion, TotalTransactions fits in int
}

// TestListTransactions_AbortedTxReportedAsTerminal proves ListTransactions reports
// an aborted tx as terminal (Failed) even though its underlying KnownTx row is left
// in a non-terminal ProvenTxReq status ('nosend'). AbortAction only flips the
// Transaction to 'aborted'; it never touches the (possibly shared) KnownTx row, so a
// tx created via ProcessAction(IsNoSend:true) and then aborted keeps KnownTx at
// 'nosend'. Before the fix, the standardized-status override in ListTransactions only
// fired for txStatusMap[txID] == TxStatusFailed, so this case fell through to the base
// KnownTx-derived status (nosend -> Waiting) - reporting a dead, input-released tx as
// still in-flight. The toolbox-owned standardized-status surface must always read
// 'aborted' as terminal; the retryable nuance lives only on the raw TxStatus.
func TestListTransactions_AbortedTxReportedAsTerminal(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and: a tx created with IsNoSend:true (real TxID + KnownTx status 'nosend')
	createResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()
	_, err := activeStorage.ProcessAction(ctx, testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		IsNoSend:  true,
		Reference: to.Ptr(createResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:     signedTx.Bytes(),
		SendWith:  []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// and: confirm the pre-abort KnownTx status really is 'nosend', so the post-list
	// assertion below pins the exact value rather than an assumption
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).WithStatus(wdk.ProvenTxStatusNoSend)

	// and: abort it - 'nosend' is abortable, and abort does not touch KnownTx
	_, err = activeStorage.AbortAction(ctx, testusers.Alice.AuthID(), wdk.AbortActionArgs{
		Reference: primitives.Base64String(createResult.Reference),
	})
	require.NoError(t, err)

	testabilities.ThenDBState(t, activeStorage).
		HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusAborted)

	// When:
	args := wdk.ListTransactionsArgs{
		Limit:  10,
		Offset: 0,
		TxIDs:  []string{txID},
	}
	result, err := activeStorage.ListTransactions(ctx, testusers.Alice.AuthID(), args)

	// Then: reported as terminal (Failed), NOT Waiting, despite the KnownTx row
	// still sitting at the non-terminal 'nosend' status
	require.NoError(t, err)
	require.Len(t, result.Transactions, 1)
	assert.Equal(t, txID, result.Transactions[0].TxID)
	assert.Equal(t, wdk.TxUpdateStatusFailed, result.Transactions[0].Status)
}
