package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.GreaterOrEqual(t, int(result.TotalTransactions), 1)
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

func TestListTransactions_FilterByReference(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// Create actions with different references
	customReference := "custom-tx-ref-123"
	given.Action(activeStorage).WithReference(customReference).Processed()
	given.Action(activeStorage).WithSatoshisToInternalize(50000).Processed()

	// When: filter by specific reference
	args := wdk.ListTransactionsArgs{
		Limit:     10,
		Offset:    0,
		Reference: &customReference,
	}
	result, err := activeStorage.ListTransactions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, primitives.PositiveInteger(1), result.TotalTransactions)
	assert.Len(t, result.Transactions, 1)
}

func TestListTransactions_FilterByReferenceNoMatch(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// Create an action without specific reference
	given.Action(activeStorage).Processed()

	// When: filter by non-existent reference
	nonExistentRef := "non-existent-reference"
	args := wdk.ListTransactionsArgs{
		Limit:     10,
		Offset:    0,
		Reference: &nonExistentRef,
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
		TxID:   &txID,
	}
	result, err := activeStorage.ListTransactions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, primitives.PositiveInteger(1), result.TotalTransactions)
	assert.Len(t, result.Transactions, 1)
	assert.Equal(t, txID, result.Transactions[0].TxID)
}

func TestListTransactions_NilReferenceReturnsAll(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// Create multiple actions
	customReference := "some-reference"
	given.Action(activeStorage).WithReference(customReference).Processed()
	given.Action(activeStorage).WithSatoshisToInternalize(50000).Processed()

	// When: no reference filter (nil)
	args := wdk.ListTransactionsArgs{
		Limit:     10,
		Offset:    0,
		Reference: nil,
	}
	result, err := activeStorage.ListTransactions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, int(result.TotalTransactions), 2)
}
