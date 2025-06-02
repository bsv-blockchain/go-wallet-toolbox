package storage_test

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/validate"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListActions_HappyPath(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and:
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	_, err := activeStorage.CreateAction(ctx, testusers.Alice.AuthID(), fixtures.DefaultValidCreateActionArgs())
	require.NoError(t, err)

	// When:
	args := wdk.ListActionsArgs{
		Limit:          10,
		Offset:         0,
		LabelQueryMode: to.Ptr(primitives.LabelQueryModeString("any")), // "any=false" or "all=true"
		IncludeLabels:  to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeOutputs: to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeInputs:  to.Ptr(primitives.BooleanDefaultFalse(true)),
	}
	result, err := activeStorage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Actions)
	assert.Greater(t, int(result.TotalActions), 0)
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

	storage := given.Provider().GORM()

	// When:
	args := wdk.ListActionsArgs{
		Limit: validate.MaxPaginationLimit + 1,
	}
	_, err := storage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid listActions args")
}

func TestListActions_EmptyResult(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	storage := given.Provider().GORM()

	expected := &wdk.ListActionsResult{
		TotalActions: 0,
		Actions:      []wdk.WalletAction{},
	}

	// When:
	args := wdk.ListActionsArgs{
		Limit:  10,
		Offset: 0,
	}
	result, err := storage.ListActions(ctx, testusers.Alice.AuthID(), args)

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

	storage := given.Provider().GORM()
	given.Faucet(storage, testusers.Alice).TopUp(100_000)

	_, err := storage.CreateAction(ctx, testusers.Alice.AuthID(), fixtures.DefaultValidCreateActionArgs())
	require.NoError(t, err)

	// When:
	args := wdk.ListActionsArgs{
		Limit:                       10,
		Offset:                      0,
		IncludeOutputs:              to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeOutputLockingScripts: to.Ptr(primitives.BooleanDefaultFalse(true)),
	}
	result, err := storage.ListActions(ctx, testusers.Alice.AuthID(), args)

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
	storage := given.Provider().GORM()
	given.Faucet(storage, testusers.Alice).TopUp(100_000)
	_, err := storage.CreateAction(ctx, testusers.Alice.AuthID(), fixtures.DefaultValidCreateActionArgs())
	require.NoError(t, err)

	// When:
	args := wdk.ListActionsArgs{
		Limit:                            10,
		Offset:                           0,
		IncludeInputs:                    to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeInputSourceLockingScripts: to.Ptr(primitives.BooleanDefaultFalse(true)),
	}
	result, err := storage.ListActions(ctx, testusers.Alice.AuthID(), args)

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
	storage := given.Provider().GORM()
	given.Faucet(storage, testusers.Alice).TopUp(100_000)
	_, err := storage.CreateAction(ctx, testusers.Alice.AuthID(), fixtures.DefaultValidCreateActionArgs())
	require.NoError(t, err)

	// When:
	args := wdk.ListActionsArgs{
		Limit:                        10,
		Offset:                       0,
		IncludeInputs:                to.Ptr(primitives.BooleanDefaultFalse(true)),
		IncludeInputUnlockingScripts: to.Ptr(primitives.BooleanDefaultFalse(true)),
	}
	result, err := storage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.NoError(t, err)
	require.NotEmpty(t, result.Actions)
	for _, action := range result.Actions {
		for _, in := range action.Inputs {
			require.NotEmpty(t, in.UnlockingScript)
		}
	}
}

func TestListActions_SeekPermissionsFalse(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	storage := given.Provider().GORM()

	// When:
	args := wdk.ListActionsArgs{
		Limit:           10,
		Offset:          0,
		SeekPermissions: to.Ptr(primitives.BooleanDefaultTrue(false)),
	}
	_, err := storage.ListActions(ctx, testusers.Alice.AuthID(), args)

	// Then:
	require.Error(t, err)
	require.ErrorContains(t, err, "seekPermissions=false")
}
