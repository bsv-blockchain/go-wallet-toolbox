package storage_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestListActions_HappyPath(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and:
	_, ownedTransaction := given.Action(activeStorage).Processed()

	// When:
	args := wdk.ListActionsArgs{
		Limit:          10,
		Offset:         0,
		IncludeLabels:  to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeOutputs: to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeInputs:  to.Ptr(primitives.BooleanDefaultFalse(true)),
	}
	result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, primitives.PositiveInteger(2), result.TotalActions)
	assert.Len(t, result.Actions, 2)

	internalizedTx := result.Actions[0]
	assert.Equal(t, ownedTransaction.Inputs[0].SourceTXID.String(), internalizedTx.TxID)
	assert.Empty(t, internalizedTx.Inputs)

	createdTx := result.Actions[1]
	assert.Equal(t, ownedTransaction.TxID().String(), createdTx.TxID)
	assert.Contains(t, createdTx.Labels, fixtures.CreateActionTestLabel)

	require.Len(t, createdTx.Inputs, 1)
	createdTxInput := createdTx.Inputs[0]
	assert.Equal(t,
		string(primitives.NewOutpointString(ownedTransaction.Inputs[0].SourceTXID.String(), ownedTransaction.Inputs[0].SourceTxOutIndex)),
		createdTxInput.SourceOutpoint,
	)

	require.Len(t, createdTx.Outputs, len(ownedTransaction.Outputs))

	resultOutput := createdTx.Outputs[0]
	assert.Contains(t, resultOutput.Tags, fixtures.CreateActionTestTag)
}

func TestListActions_InvalidAuth(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	gormProvider := given.Provider().GORM()
	args := wdk.ListActionsArgs{
		Limit:  10,
		Offset: 0,
	}

	// When:
	_, err := gormProvider.ListActions(ctx, wdk.AuthID{UserID: nil}, args)

	// Then:
	require.ErrorIs(t, err, storage.ErrAuthorization)
}

func TestListActions_InvalidArgs(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// When:
	args := wdk.ListActionsArgs{
		Limit: validate.MaxPaginationLimit + 1,
	}
	_, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid listActions args")
}

func TestListActions_EmptyResult(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	expected := &wdk.ListActionsResult{
		TotalActions: 0,
		Actions:      []wdk.WalletAction{},
	}

	// When:
	args := wdk.ListActionsArgs{
		Limit:  10,
		Offset: 0,
	}
	result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, expected, result)
}

func TestListActions_IncludeLabelsAndOutputs(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	faucet := given.Faucet(activeStorage, testusers.Alice)
	faucet.TopUp(100_000)

	_, err := activeStorage.CreateAction(ctx, testusers.Alice.AuthID(), fixtures.DefaultValidCreateActionArgs())
	require.NoError(t, err)

	args := wdk.ListActionsArgs{
		Limit:          10,
		Offset:         0,
		IncludeLabels:  to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeOutputs: to.Ptr(primitives.BooleanDefaultFalse(true)),
	}

	// When:
	result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotEmpty(t, result.Actions)
	for _, action := range result.Actions {
		require.NotNil(t, action.Labels)
		require.NotNil(t, action.Outputs)
	}
}

func TestListActions_IncludeOutputLockingScripts(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	_, err := activeStorage.CreateAction(ctx, testusers.Alice.AuthID(), fixtures.DefaultValidCreateActionArgs())
	require.NoError(t, err)

	// When:
	args := wdk.ListActionsArgs{
		Limit:                       10,
		Offset:                      0,
		IncludeOutputs:              to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeOutputLockingScripts: to.Ptr(primitives.BooleanDefaultFalse(true)),
	}
	result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotEmpty(t, result.Actions)

	found := false
	for _, action := range result.Actions {
		for _, out := range action.Outputs {
			if out.LockingScript != "" {
				found = true
			}
		}
	}
	require.True(t, found, "Expected at least one output with a locking script")
}

func TestListActions_IncludeInputSourceLockingScripts(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().GORM()
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	_, err := activeStorage.CreateAction(ctx, testusers.Alice.AuthID(), fixtures.DefaultValidCreateActionArgs())
	require.NoError(t, err)

	// When:
	args := wdk.ListActionsArgs{
		Limit:                            10,
		Offset:                           0,
		IncludeInputs:                    to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeInputSourceLockingScripts: to.Ptr(primitives.BooleanDefaultFalse(true)),
	}
	result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotEmpty(t, result.Actions)
	for _, action := range result.Actions {
		for _, in := range action.Inputs {
			require.NotEmpty(t, in.SourceLockingScript)
		}
	}
}

func TestListActions_IncludeInputUnlockingScripts(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().GORM()
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	_, err := activeStorage.CreateAction(ctx, testusers.Alice.AuthID(), fixtures.DefaultValidCreateActionArgs())
	require.NoError(t, err)

	// When:
	args := wdk.ListActionsArgs{
		Limit:                        10,
		Offset:                       0,
		IncludeInputs:                to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeInputUnlockingScripts: to.Ptr(primitives.BooleanDefaultFalse(true)),
	}
	result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotEmpty(t, result.Actions)
	for _, action := range result.Actions {
		for _, in := range action.Inputs {
			require.NotEmpty(t, in.UnlockingScript)
		}
	}
}

func TestListActions_SeekPermissionFalse(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().GORM()

	// When:
	args := wdk.ListActionsArgs{
		Limit:          10,
		Offset:         0,
		SeekPermission: to.Ptr(primitives.BooleanDefaultTrue(false)),
	}
	_, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.Error(t, err)
	require.ErrorContains(t, err, "seekPermission=false")
}
