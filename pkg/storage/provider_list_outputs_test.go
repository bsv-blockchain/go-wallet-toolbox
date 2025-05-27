package storage_test

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListOutputs_EmptyByDefault(t *testing.T) {
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

func TestListOutputs_IncludeTransactions(t *testing.T) {
	// Given:
	ctx := t.Context()
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	actionResult, signedTx := given.ActionCreatedAndSigned(activeStorage)

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
	require.Len(t, actualResult.Outputs, 33)

	// and:
	beef := testutils.BEEFFromHex(t, *actualResult.BEEF)
	require.Len(t, beef.Transactions, 2) // parent transaction with BUMP and the internalized one (with no BUMP)

	// Given:
	createdTxID := signedTx.TxID().String()
	processArgs := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  &actionResult.Reference,
		TxID:       (*primitives.TXIDHexString)(&createdTxID),
		RawTx:      signedTx.Bytes(),
		SendWith:   []string{},
	}
	_, err = activeStorage.ProcessAction(ctx, testusers.Alice.AuthID(), processArgs)
	require.NoError(t, err)

	// When:
	actualResult, err = activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, actualResult)
	require.Len(t, actualResult.Outputs, 33)

	// and:
	require.NotNil(t, actualResult.BEEF)
	beef = testutils.BEEFFromHex(t, *actualResult.BEEF)
	require.Len(t, beef.Transactions, 3) // parent transaction with BUMP, the internalized one (with no BUMP), AND the newly created transaction

	for _, output := range actualResult.Outputs {
		require.NotEmpty(t, output.Outpoint)
		require.NoError(t, output.Outpoint.Validate())
		require.NotNil(t, beef.FindTransaction(output.Outpoint.MustGetTxID()))
	}
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
	assert.Greater(t, int(actualResult.TotalOutputs), 0, "Expected totalOutputs > 0 for basket %s", basketName)
}
