package methodtests

import (
	"context"
	"encoding/hex"
	"slices"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/actions/funder/errfunder"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testutils"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateActionNilAuth(t *testing.T) {
	given := testabilities.Given(t)

	// given:
	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.CreateAction(context.Background(), wdk.AuthID{UserID: nil}, fixtures.DefaultValidCreateActionArgs())

	// then:
	require.Error(t, err)
}

func TestCreateActionHappyPath(t *testing.T) {
	given := testabilities.Given(t)

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	providedOutput := args.Outputs[0]

	// when:
	result, err := activeStorage.CreateAction(
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 24, len(result.DerivationPrefix))
	assert.Equal(t, 16, len(result.Reference))
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Equal(t, 32, len(result.Outputs))
	assert.Equal(t, 31, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(57_998), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))
	assert.Equal(t, "0200beef0000", hex.EncodeToString(result.InputBeef))

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	resultOutput := result.Outputs[0]

	require.Equal(t, wdk.ProvidedByYou, resultOutput.ProvidedBy)
	assert.Empty(t, resultOutput.Purpose)
	assert.Equal(t, providedOutput.Satoshis, resultOutput.Satoshis)
	assert.Equal(t, providedOutput.Basket, resultOutput.Basket)
	assert.Equal(t, providedOutput.LockingScript, resultOutput.LockingScript)
	assert.Equal(t, providedOutput.CustomInstructions, resultOutput.CustomInstructions)
	assert.Equal(t, providedOutput.Tags, resultOutput.Tags)

	input := result.Inputs[0]
	assert.Equal(t, 1, len(result.Inputs))
	assert.Equal(t, 0, input.Vin)
	assert.NotEmpty(t, input.SourceTxID)
	assert.Equal(t, uint32(0), input.SourceVout)
	assert.Equal(t, int64(100_000), input.SourceSatoshis)
	assert.NotEmpty(t, input.SourceLockingScript)
	assert.Nil(t, input.SourceTransaction)
	assert.Equal(t, wdk.ProvidedByStorage, input.ProvidedBy)
	assert.Equal(t, string(wdk.OutputTypeP2PKH), input.Type)
	require.NotEmpty(t, input.DerivationPrefix)
	assert.Equal(t, 24, len(*input.DerivationPrefix))
	require.NotEmpty(t, input.DerivationSuffix)
	assert.Equal(t, 24, len(*input.DerivationSuffix))

	// TODO: Test DB state: but after we make actual getter methods, like ListActions
}

func TestCreateActionWithSignActionHappyPath(t *testing.T) {
	given := testabilities.Given(t)

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsSignAction = true
	args.Options.SignAndProcess = to.Ptr[primitives.BooleanDefaultTrue](false)

	// when:
	result, err := activeStorage.CreateAction(
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	input := result.Inputs[0]
	require.NotEmpty(t, input.SourceTransaction)
}

func TestCreateActionWithCommission(t *testing.T) {
	given := testabilities.Given(t)

	// given:
	activeStorage := given.Provider().
		WithCommission(defs.Commission{
			PubKeyHex: "03398d26f180996f8a2cb175a99620630d76257ccfef4ac7d303c8aa6f90c3190c",
			Satoshis:  10,
		}).
		GORM()

	// and:
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()

	// when:
	result, err := activeStorage.CreateAction(context.Background(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 24, len(result.DerivationPrefix))
	assert.Equal(t, 16, len(result.Reference))
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Equal(t, 33, len(result.Outputs))
	assert.Equal(t, 32, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(57_998), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))
	assert.Equal(t, "0200beef0000", hex.EncodeToString(result.InputBeef))

	commissionOutput, _ := testutils.FindOutput(t, result.Outputs, testutils.CommissionOutputCondition)
	assert.Equal(t, primitives.SatoshiValue(10), commissionOutput.Satoshis)
	assert.Nil(t, commissionOutput.Basket)
	assert.Equal(t, wdk.ProvidedByStorage, commissionOutput.ProvidedBy)
	assert.Nil(t, commissionOutput.DerivationSuffix)
	assert.NotEmpty(t, commissionOutput.LockingScript)
	assert.NoError(t, commissionOutput.LockingScript.Validate())
	assert.Empty(t, commissionOutput.OutputDescription)
	assert.Nil(t, commissionOutput.CustomInstructions)
	assert.Empty(t, commissionOutput.Tags)
}

func TestCreateActionShuffleOutputs(t *testing.T) {
	given := testabilities.Given(t)

	// given:
	activeStorage := given.Provider().
		WithCommission(defs.Commission{
			PubKeyHex: "03398d26f180996f8a2cb175a99620630d76257ccfef4ac7d303c8aa6f90c3190c",
			Satoshis:  10,
		}).
		GORM()

	// and:
	faucet := given.Faucet(activeStorage, testusers.Alice)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.Options.RandomizeOutputs = true

	commissionOutputVouts := map[uint32]struct{}{}
	for range 100 {
		// when:
		faucet.TopUp(100_000)

		result, _ := activeStorage.CreateAction(
			context.Background(),
			testusers.Alice.AuthID(),
			args,
		)

		found := slices.IndexFunc(result.Outputs, testutils.CommissionOutputCondition)
		commissionOutputVouts[result.Outputs[found].Vout] = struct{}{}

		if len(commissionOutputVouts) > 1 {
			t.Log("Random shuffle works! Found commission outputs at different vouts")
			return
		}
	}

	t.Error("Expected commission output to be shuffled, but it was not")
}

func TestZeroFunds(t *testing.T) {
	given := testabilities.Given(t)

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	args := fixtures.DefaultValidCreateActionArgs()

	// when:
	_, err := activeStorage.CreateAction(
		context.Background(),
		testusers.Bob.AuthID(),
		args,
	)

	// then:
	require.Error(t, err)
}

func TestInsufficientFunds(t *testing.T) {
	given := testabilities.Given(t)

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	given.Faucet(activeStorage, testusers.Alice).TopUp(1)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()

	// when:
	_, err := activeStorage.CreateAction(
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.Error(t, err)
}

func TestReservedUTXO(t *testing.T) {
	given := testabilities.Given(t)

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()

	// when:
	_, err := activeStorage.CreateAction(
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)

	// when:
	_, err = activeStorage.CreateAction(
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.ErrorIs(t, err, errfunder.NotEnoughFunds)
}
