package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
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
