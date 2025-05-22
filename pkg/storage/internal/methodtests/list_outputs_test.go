package methodtests

import (
	"strings"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListOutputs_EmptyByDefault(t *testing.T) {
	// given:
	ctx := t.Context()
	given := testabilities.Given(t)
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

func TestListOutputs_WithKnownTxidsAndIncludeTransactions(t *testing.T) {
	// Given:
	ctx := t.Context()
	given := testabilities.Given(t)
	activeStorage := given.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	actionResult, signedTx := given.ActionCreatedAndSigned(activeStorage)
	txid := signedTx.TxID().String()

	processArgs := wdk.ProcessActionArgs{
		IsNewTx:    true,
		IsSendWith: false,
		IsNoSend:   false,
		IsDelayed:  false,
		Reference:  &actionResult.Reference,
		TxID:       (*primitives.TXIDHexString)(&txid),
		RawTx:      signedTx.Bytes(),
		SendWith:   []string{},
	}
	_, err := activeStorage.ProcessAction(ctx, testusers.Alice.AuthID(), processArgs)
	require.NoError(t, err)

	listArgs := wdk.ListOutputsArgs{
		Basket:              "",
		Limit:               10,
		Offset:              0,
		KnownTxids:          []string{txid},
		IncludeTransactions: true,
	}

	// When:
	actualResult, err := activeStorage.ListOutputs(ctx, testusers.Alice.AuthID(), listArgs)

	// Then:
	require.NoError(t, err)
	require.NotNil(t, actualResult)
	require.NotEmpty(t, actualResult.Outputs)
	require.NotNil(t, actualResult.BEEF)

	expectedOutpoints := map[string]bool{}
	for _, out := range actualResult.Outputs {
		parts := strings.Split(string(out.Outpoint), ".")
		require.Len(t, parts, 2, "Outpoint format should be <txid>.<vout>")
		require.Equal(t, txid, parts[0], "Output txid should match known txid")
		expectedOutpoints[string(out.Outpoint)] = true
	}

	for _, out := range actualResult.Outputs {
		assert.Truef(t, expectedOutpoints[string(out.Outpoint)], "Unexpected outpoint: %s", out.Outpoint)
	}
}

func TestListOutputs_FilterByBasketName(t *testing.T) {
	// Given:
	ctx := t.Context()
	given := testabilities.Given(t)
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
