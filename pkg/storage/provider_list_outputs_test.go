package storage_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestListOutputs_MinimalFilter(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	given.Action(activeStorage).Processed()

	listArgs := wdk.ListOutputsArgs{
		Limit: 100,
	}

	// when:
	result, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Outputs, 9)
	require.Equal(t, primitives.PositiveInteger(9), result.TotalOutputs)

	// and:
	require.Nil(t, result.BEEF)

	// and:
	for _, output := range result.Outputs {
		assert.NotEmpty(t, output.Outpoint)
		require.NoError(t, output.Outpoint.Validate())
		assert.NotEqual(t, primitives.SatoshiValue(0), output.Satoshis)

		assert.Empty(t, output.LockingScript)
		assert.Empty(t, output.Tags)
		assert.Empty(t, output.Labels)
		assert.Empty(t, output.CustomInstructions)
	}
}

func TestListOutputs_IncludeTags(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and:
	internalizeArgs := fixtures.DefaultInternalizeActionArgs(t, wdk.BasketInsertionProtocol)

	internalizeResult, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		internalizeArgs,
	)
	require.NoError(t, err)

	// when:
	listArgs := wdk.ListOutputsArgs{
		Limit:       100,
		IncludeTags: true,
	}
	result, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// Then:
	require.NoError(t, err)

	// and:
	outpoint := primitives.NewOutpointString(internalizeResult.TxID, 0)
	output, _ := testutils.FindOutput(t, result.Outputs, func(p *wdk.WalletOutput) bool {
		return p.Outpoint == outpoint
	})

	for _, tag := range internalizeArgs.Outputs[0].InsertionRemittance.Tags {
		assert.Contains(t, output.Tags, tag)
	}
}

func TestListOutputs_IncludeCustomInstructions(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and:
	internalizeArgs := fixtures.DefaultInternalizeActionArgs(t, wdk.BasketInsertionProtocol)

	internalizeResult, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		internalizeArgs,
	)
	require.NoError(t, err)

	// when:
	listArgs := wdk.ListOutputsArgs{
		Limit:                     100,
		IncludeCustomInstructions: true,
	}
	result, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// Then:
	require.NoError(t, err)

	// and:
	outpoint := primitives.NewOutpointString(internalizeResult.TxID, 0)
	output, _ := testutils.FindOutput(t, result.Outputs, func(p *wdk.WalletOutput) bool {
		return p.Outpoint == outpoint
	})

	assert.Equal(t, to.Value(internalizeArgs.Outputs[0].InsertionRemittance.CustomInstructions), *output.CustomInstructions)
}

func TestListOutputs_IncludeLockingScripts(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	given.Action(activeStorage).Processed()

	listArgs := wdk.ListOutputsArgs{
		Limit:                 100,
		IncludeLockingScripts: true,
	}

	// when:
	result, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// Then:
	require.NoError(t, err)

	// and:
	for _, output := range result.Outputs {
		assert.NotEmpty(t, output.LockingScript)
	}
}

func TestListOutputs_IncludeTransactions(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	given.Action(activeStorage).Processed()

	listArgs := wdk.ListOutputsArgs{
		Basket:              "",
		Limit:               100,
		Offset:              0,
		IncludeTransactions: true,
	}

	// When:
	actualResult, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, actualResult)
	require.Len(t, actualResult.Outputs, 9)

	// and:
	require.NotNil(t, actualResult.BEEF)
	beef := testutils.BEEFFromBytes(t, actualResult.BEEF)
	require.Len(t, beef.Transactions, 3) // parent transaction with BUMP, the internalized one (with no BUMP), AND the newly created transaction

	// and:
	for _, output := range actualResult.Outputs {
		assert.NotEmpty(t, output.Outpoint)
		require.NoError(t, output.Outpoint.Validate())
		assert.NotNil(t, beef.FindTransaction(output.Outpoint.MustGetTxID()))
	}
}

func TestListOutputs_BeforeProcessAction(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and:
	given.Action(activeStorage).WithSatoshisToInternalize(fixtures.DefaultCreateActionOutputSatoshis).Created()

	listArgs := wdk.ListOutputsArgs{
		Basket:              "",
		Limit:               100,
		Offset:              0,
		IncludeTransactions: true,
	}

	// when:
	actualResult, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// then:
	require.NoError(t, err)
	require.NotNil(t, actualResult)
	require.Len(t, actualResult.Outputs, 9)

	// and:
	beef := testutils.BEEFFromBytes(t, actualResult.BEEF)
	require.Empty(t, beef.Transactions)
}

func TestListOutputs_FilterTags(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and:
	faucet := given.Faucet(activeStorage, testusers.Alice)
	faucet.TopUp(1000)
	faucet.TopUp(1001)

	listArgs := wdk.ListOutputsArgs{
		Basket:      "",
		Limit:       100,
		Offset:      0,
		IncludeTags: true,
		Tags: []primitives.StringUnder300{
			primitives.StringUnder300(fixtures.FaucetTag(0)),
		},
	}

	// when:
	actualResult, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// then:
	require.NoError(t, err)
	require.Len(t, actualResult.Outputs, 1)
	require.Equal(t, primitives.PositiveInteger(1), actualResult.TotalOutputs)

	foundOutput := actualResult.Outputs[0]
	assert.Contains(t, foundOutput.Tags, primitives.StringUnder300(fixtures.CreateActionTestTag))
	assert.Contains(t, foundOutput.Tags, primitives.StringUnder300(fixtures.FaucetTag(0)))
}

func TestListOutputs_FilterTagsAllMode(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and:
	faucet := given.Faucet(activeStorage, testusers.Alice)
	faucet.TopUp(1000)
	faucet.TopUp(1001)

	listArgs := wdk.ListOutputsArgs{
		Basket:      "",
		Limit:       100,
		Offset:      0,
		IncludeTags: true,
		Tags: []primitives.StringUnder300{
			fixtures.CreateActionTestTag,
			primitives.StringUnder300(fixtures.FaucetTag(1)),
		},
		TagQueryMode: to.Ptr(defs.QueryModeAll),
	}

	// when:
	actualResult, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// then:
	require.NoError(t, err)
	require.Len(t, actualResult.Outputs, 1)

	// and:
	foundOutput := actualResult.Outputs[0]
	assert.Contains(t, foundOutput.Tags, primitives.StringUnder300(fixtures.CreateActionTestTag))
	assert.Contains(t, foundOutput.Tags, primitives.StringUnder300(fixtures.FaucetTag(1)))
}

func TestListOutputs_FilterByBasketName(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	user := testusers.Alice
	faucet := given.Faucet(activeStorage, user)
	_, _ = faucet.TopUp(1000)

	basketName := wdk.BasketNameForChange

	// When:
	listArgs := wdk.ListOutputsArgs{
		Basket: primitives.StringUnder300(basketName),
		Limit:  10,
		Offset: 0,
	}
	actualResult, err := activeStorage.ListOutputs(ctx, user.AuthID(), listArgs)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, actualResult)
	require.NotEmpty(t, actualResult.Outputs, "Expected outputs for basket %s", basketName)
	assert.Positive(t, int(actualResult.TotalOutputs), "Expected totalOutputs > 0 for basket %s", basketName) //nolint:gosec // test assertion, totalOutputs fits in int
}

func TestListOutputs_NoOutputsToReturn(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	args := wdk.ListOutputsArgs{
		Basket: "",
		Limit:  10,
		Offset: 0,
	}

	// when:
	actualResult, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), args)

	expectedResult := &wdk.ListOutputsResult{
		TotalOutputs: 0,
		BEEF:         nil,
		Outputs:      []*wdk.WalletOutput{},
	}
	// then:
	require.NoError(t, err)
	require.NotNil(t, actualResult)
	require.Equal(t, expectedResult, actualResult)
}

func TestListOutputs_ShouldReturnOnlySpendableOutputs(t *testing.T) {
	// given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	createActionRes, _ := given.Action(activeStorage).Processed()

	changeOutputs := seq.Filter(seq.FromSlice(createActionRes.Outputs), func(output *wdk.StorageCreateTransactionSdkOutput) bool {
		return output.Purpose == wdk.ChangePurpose && output.ProvidedBy == wdk.ProvidedByStorage
	})

	balance := satoshi.Value(0)
	for output := range changeOutputs {
		balance = satoshi.MustAdd(balance, output.Satoshis)
	}

	changeOutputsCount := seq.Count(changeOutputs)

	args := wdk.ListOutputsArgs{
		Basket: wdk.BasketNameForChange,
		Limit:  1000,
		Offset: 0,
	}

	// when:
	result, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)
	require.Len(t, result.Outputs, changeOutputsCount)

	// when:
	createActionArgs := fixtures.DefaultValidCreateActionArgs()
	createActionArgs.Outputs[0].Satoshis = primitives.SatoshiValue(satoshi.MustSubtract(balance, 5).Int64()) //nolint:gosec // satoshi value is always non-negative
	_, err = activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		createActionArgs,
	)

	// then:
	require.NoError(t, err)

	// when:
	result, err = activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)
	require.LessOrEqual(t, len(result.Outputs), 1)
	if len(result.Outputs) == 1 {
		require.Less(t, result.Outputs[0].Satoshis, primitives.SatoshiValue(5))
	}
}
