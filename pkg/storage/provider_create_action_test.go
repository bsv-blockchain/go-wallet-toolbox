package storage_test

import (
	"slices"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder/errfunder"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestCreateActionNilAuth(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.CreateAction(t.Context(), wdk.AuthID{UserID: nil}, fixtures.DefaultValidCreateActionArgs())

	// then:
	require.Error(t, err)
}

func TestCreateActionHappyPath(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	faucetTx, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	providedOutput := args.Outputs[0]

	// when:
	result, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	assert.Len(t, result.DerivationPrefix, 24)
	assert.Len(t, result.Reference, 16)
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Len(t, result.Outputs, 9)
	assert.Equal(t, 8, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(57999), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))

	pkgtestabilities.AssertBEEFState(t, result.InputBeef, pkgtestabilities.ExpectedBeefTransactionState{
		ID: faucetTx.ID().String(),
	})

	assert.Empty(t, result.NoSendChangeOutputVouts)

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p *wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	resultOutput := result.Outputs[0]

	require.Equal(t, wdk.ProvidedByYou, resultOutput.ProvidedBy)
	assert.Empty(t, resultOutput.Purpose)
	assert.Equal(t, providedOutput.Satoshis, resultOutput.Satoshis)
	assert.Equal(t, providedOutput.Basket, resultOutput.Basket)
	assert.Equal(t, providedOutput.LockingScript, resultOutput.LockingScript)
	assert.Equal(t, providedOutput.CustomInstructions, resultOutput.CustomInstructions)
	assert.Contains(t, resultOutput.Tags, primitives.StringUnder300(fixtures.CreateActionTestTag))

	require.Len(t, result.Inputs, 1)
	input := result.Inputs[0]
	assert.Equal(t, 0, input.Vin)
	assert.NotEmpty(t, input.SourceTxID)
	assert.Equal(t, uint32(0), input.SourceVout)
	assert.Equal(t, int64(100_000), input.SourceSatoshis)
	assert.NotEmpty(t, input.SourceLockingScript)
	assert.Nil(t, input.SourceTransaction)
	assert.Equal(t, wdk.ProvidedByStorage, input.ProvidedBy)
	assert.Equal(t, wdk.OutputTypeP2PKH, input.Type)
	require.NotEmpty(t, input.DerivationPrefix)
	require.NotEmpty(t, input.DerivationSuffix)

	// TODO: Test DB state: but after we make actual getter methods, like ListActions
}

func TestCreateActionWithNoSendChangeHappyPath(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	opts := []pkgtestabilities.TopUpOpts{
		pkgtestabilities.WithPurpose(wdk.ChangePurpose),
	}

	transactionSpec1, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000, opts...)
	transactionSpec2, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(200_000, opts...)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsNoSend = true
	args.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(true))
	args.Options.NoSendChange = []wdk.OutPoint{
		{
			TxID: transactionSpec1.ID().String(),
			Vout: 0,
		},
		{
			TxID: transactionSpec2.ID().String(),
			Vout: 0,
		},
	}

	// when:
	result, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	assert.NotNil(t, result)
	const (
		firstNoSendChangeVout = 1
		lastNoSendChangeVout  = 8
	)
	expectedNoSendChangeOutputs := seq.Collect(seq.Range(firstNoSendChangeVout, lastNoSendChangeVout+1)) // [firstNoSendChangeVout ... lastNoSendChangeVout]
	assert.Equal(t, expectedNoSendChangeOutputs, result.NoSendChangeOutputVouts)
}

func TestCreateActionWithNoSendChangeDuplicate(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	transactionSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000, pkgtestabilities.WithPurpose(wdk.ChangePurpose))

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsNoSend = true
	args.Options.NoSend = to.Ptr(primitives.BooleanDefaultFalse(true))
	outpoint := wdk.OutPoint{
		TxID: transactionSpec.ID().String(),
		Vout: 0,
	}

	args.Options.NoSendChange = []wdk.OutPoint{
		outpoint, outpoint, // NOTE: duplicate outpoints
	}

	// when:
	_, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.Error(t, err)
}

func TestCreateActionOutputTags(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:

	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	const tag1 = "tag1"
	const tag2 = "tag2"
	args := fixtures.DefaultValidCreateActionArgs()
	providedOutput := &args.Outputs[0]
	providedOutput.Tags = []primitives.StringUnder300{tag1, tag2}

	// when:
	result, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)

	resultOutput := result.Outputs[0]
	assert.Equal(t, providedOutput.Tags, resultOutput.Tags)

	for _, providedByStorageOutput := range result.Outputs[1:] {
		assert.NotContains(t, providedByStorageOutput.Tags, tag1)
		assert.NotContains(t, providedByStorageOutput.Tags, tag2)
	}
}

func TestCreateActionWithSignActionHappyPath(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

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
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	input := result.Inputs[0]
	require.NotEmpty(t, input.SourceTransaction)
}

func TestCreateActionWithCommission(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().
		WithCommission(defs.Commission{
			PubKeyHex: "03398d26f180996f8a2cb175a99620630d76257ccfef4ac7d303c8aa6f90c3190c",
			Satoshis:  10,
		}).
		GORM()

	// and:
	faucetTx, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()

	// when:
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), args)

	// then:
	require.NoError(t, err)
	assert.Len(t, result.DerivationPrefix, 24)
	assert.Len(t, result.Reference, 16)
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Len(t, result.Outputs, 10)
	assert.Equal(t, 9, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(57999), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))

	pkgtestabilities.AssertBEEFState(t, result.InputBeef, pkgtestabilities.ExpectedBeefTransactionState{
		ID: faucetTx.ID().String(),
	})

	commissionOutput, _ := testutils.FindOutput(t, result.Outputs, testutils.CommissionOutputCondition)
	assert.Equal(t, primitives.SatoshiValue(10), commissionOutput.Satoshis)
	assert.Nil(t, commissionOutput.Basket)
	assert.Equal(t, wdk.ProvidedByStorage, commissionOutput.ProvidedBy)
	assert.Nil(t, commissionOutput.DerivationSuffix)
	assert.NotEmpty(t, commissionOutput.LockingScript)
	require.NoError(t, commissionOutput.LockingScript.Validate())
	assert.Empty(t, commissionOutput.OutputDescription)
	assert.Nil(t, commissionOutput.CustomInstructions)
	assert.Empty(t, commissionOutput.Tags)

	// when:
	commissions, err := activeStorage.CommissionEntity().Read().
		IsRedeemed(false).
		Satoshis().Equals(10).
		UserID(testusers.Alice.ID).
		Find(t.Context())

	// then:
	require.NoError(t, err)
	require.Len(t, commissions, 1)
}

func TestCreateActionShuffleOutputs(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

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
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		found := slices.IndexFunc(result.Outputs, testutils.CommissionOutputCondition)
		commissionOutputVouts[result.Outputs[found].Vout] = struct{}{}

		if len(commissionOutputVouts) > 1 {
			t.Log("Random shuffle works! Found Commission outputs at different vouts")
			return
		}
	}

	t.Error("Expected Commission output to be shuffled, but it was not")
}

func TestZeroFunds(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	args := fixtures.DefaultValidCreateActionArgs()

	// when:
	_, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Bob.AuthID(),
		args,
	)

	// then:
	require.Error(t, err)
}

func TestInsufficientFunds(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	given.Faucet(activeStorage, testusers.Alice).TopUp(1)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()

	// when:
	_, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.Error(t, err)
}

func TestReservedUTXO(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()

	// when:
	_, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)

	// when:
	_, err = activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.ErrorIs(t, err, errfunder.ErrNotEnoughFunds)
}

func TestCreateActionWithProvidedKnownInput(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	ownedTx := ownedTxSpec.TX()

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsSignAction = true
	args.Options.TrustSelf = to.Ptr(sdk.TrustSelfKnown)
	args.Outputs = []wdk.ValidCreateActionOutput{}
	args.Inputs = []wdk.ValidCreateActionInput{{
		Outpoint: wdk.OutPoint{
			TxID: ownedTx.TxID().String(),
			Vout: 0,
		},
		UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
		InputDescription:      "provided input",
	}}

	// when:
	result, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	assert.Len(t, result.DerivationPrefix, 24)
	assert.Len(t, result.Reference, 16)
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Len(t, result.Outputs, 8)
	assert.Equal(t, 8, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(99999), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))

	pkgtestabilities.AssertBEEFState(t, result.InputBeef, pkgtestabilities.ExpectedBeefTransactionState{
		ID: ownedTxSpec.ID().String(),
	})

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p *wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	require.Len(t, result.Inputs, 1)
	input := result.Inputs[0]
	assert.Equal(t, 0, input.Vin)
	assert.Equal(t, input.SourceTxID, ownedTx.TxID().String())
	assert.Equal(t, uint32(0), input.SourceVout)
	assert.Equal(t, int64(100_000), input.SourceSatoshis)
	assert.NotEmpty(t, input.SourceLockingScript)
	assert.NotEmpty(t, input.SourceTransaction)
	assert.Equal(t, wdk.ProvidedByYouAndStorage, input.ProvidedBy)
	assert.Equal(t, wdk.OutputTypeP2PKH, input.Type)
	require.NotEmpty(t, input.DerivationPrefix)
	require.NotEmpty(t, input.DerivationSuffix)
}

func TestCreateActionWithProvidedUnknownInput(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	unknownParentTx := txtestabilities.GivenTX().
		WithInput(100_002).
		WithP2PKHOutput(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsSignAction = true
	args.Options.TrustSelf = to.Ptr(sdk.TrustSelfKnown)
	args.Outputs = []wdk.ValidCreateActionOutput{}
	args.Inputs = []wdk.ValidCreateActionInput{{
		Outpoint: wdk.OutPoint{
			TxID: unknownParentTx.ID().String(),
			Vout: 0,
		},
		UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
		InputDescription:      "provided unknown-by-storage input",
	}}
	args.InputBEEF = unknownParentTx.BEEF().Bytes()

	// when:
	result, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	assert.Len(t, result.DerivationPrefix, 24)
	assert.Len(t, result.Reference, 16)
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Len(t, result.Outputs, 8)
	assert.Equal(t, 8, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(99999), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))

	pkgtestabilities.AssertBEEFState(t, result.InputBeef, pkgtestabilities.ExpectedBeefTransactionState{
		ID: unknownParentTx.ID().String(),
	})

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p *wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	require.Len(t, result.Inputs, 1)
	input := result.Inputs[0]
	assert.Equal(t, 0, input.Vin)
	assert.Equal(t, input.SourceTxID, unknownParentTx.ID().String())
	assert.Equal(t, uint32(0), input.SourceVout)
	assert.Equal(t, int64(100_000), input.SourceSatoshis)
	assert.NotEmpty(t, input.SourceLockingScript)
	assert.Empty(t, input.SourceTransaction)
	assert.Equal(t, wdk.ProvidedByYou, input.ProvidedBy)
	assert.Equal(t, wdk.OutputTypeCustom, input.Type)
	assert.Nil(t, input.DerivationPrefix)
	assert.Nil(t, input.DerivationSuffix)
}

func TestCreateActionWithProvidedInputAndSmallerOutput(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	unknownParentTx := txtestabilities.GivenTX().
		WithInput(100_002).
		WithP2PKHOutput(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsSignAction = true
	args.Options.TrustSelf = to.Ptr(sdk.TrustSelfKnown)
	args.Inputs = []wdk.ValidCreateActionInput{{
		Outpoint: wdk.OutPoint{
			TxID: unknownParentTx.ID().String(),
			Vout: 0,
		},
		UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
		InputDescription:      "provided unknown-by-storage input",
	}}
	args.InputBEEF = unknownParentTx.BEEF().Bytes()

	// when:
	result, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	assert.Len(t, result.DerivationPrefix, 24)
	assert.Len(t, result.Reference, 16)
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Len(t, result.Outputs, 9)
	assert.Equal(t, 8, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(57999), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))

	pkgtestabilities.AssertBEEFState(t, result.InputBeef, pkgtestabilities.ExpectedBeefTransactionState{
		ID: unknownParentTx.ID().String(),
	})

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p *wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	require.Len(t, result.Inputs, 1)
	input := result.Inputs[0]
	assert.Equal(t, 0, input.Vin)
	assert.Equal(t, input.SourceTxID, unknownParentTx.ID().String())
	assert.Equal(t, uint32(0), input.SourceVout)
	assert.Equal(t, int64(100_000), input.SourceSatoshis)
	assert.NotEmpty(t, input.SourceLockingScript)
	assert.Empty(t, input.SourceTransaction)
	assert.Equal(t, wdk.ProvidedByYou, input.ProvidedBy)
	assert.Equal(t, wdk.OutputTypeCustom, input.Type)
	assert.Nil(t, input.DerivationPrefix)
	assert.Nil(t, input.DerivationSuffix)
}

func TestCreateActionWithProvidedInputAndGreaterOutput(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(25_000)

	unknownParentTx := txtestabilities.GivenTX().
		WithInput(25_002).
		WithP2PKHOutput(25_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsSignAction = true
	args.Options.TrustSelf = to.Ptr(sdk.TrustSelfKnown)
	args.Inputs = []wdk.ValidCreateActionInput{{
		Outpoint: wdk.OutPoint{
			TxID: unknownParentTx.ID().String(),
			Vout: 0,
		},
		UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
		InputDescription:      "provided unknown-by-storage input",
	}}
	args.InputBEEF = unknownParentTx.BEEF().Bytes()

	// when:
	result, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	assert.Len(t, result.DerivationPrefix, 24)
	assert.Len(t, result.Reference, 16)
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Len(t, result.Outputs, 9)
	assert.Equal(t, 8, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(7999), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))

	pkgtestabilities.AssertBEEFState(t, result.InputBeef, pkgtestabilities.ExpectedBeefTransactionState{
		ID: ownedTxSpec.ID().String(),
	})

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p *wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	require.Len(t, result.Inputs, 2)
	providedInput := result.Inputs[0]
	assert.Equal(t, 0, providedInput.Vin)
	assert.Equal(t, providedInput.SourceTxID, unknownParentTx.ID().String())
	assert.Equal(t, uint32(0), providedInput.SourceVout)
	assert.Equal(t, int64(25_000), providedInput.SourceSatoshis)
	assert.NotEmpty(t, providedInput.SourceLockingScript)
	assert.Empty(t, providedInput.SourceTransaction)
	assert.Equal(t, wdk.ProvidedByYou, providedInput.ProvidedBy)
	assert.Equal(t, wdk.OutputTypeCustom, providedInput.Type)
	assert.Nil(t, providedInput.DerivationPrefix)
	assert.Nil(t, providedInput.DerivationSuffix)

	allocatedInput := result.Inputs[1]
	assert.Equal(t, 1, allocatedInput.Vin)
	assert.Equal(t, allocatedInput.SourceTxID, ownedTxSpec.ID().String())
	assert.Equal(t, uint32(0), allocatedInput.SourceVout)
	assert.Equal(t, int64(25_000), allocatedInput.SourceSatoshis)
	assert.Equal(t, wdk.ProvidedByStorage, allocatedInput.ProvidedBy)
	assert.Equal(t, wdk.OutputTypeP2PKH, allocatedInput.Type)
	assert.NotEmpty(t, allocatedInput.DerivationPrefix)
	assert.NotEmpty(t, allocatedInput.DerivationSuffix)
}

func TestCreateActionWithProvidedUnknownInputWithoutInputBEEF(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	unknownParentTx := txtestabilities.GivenTX().
		WithInput(100_002).
		WithP2PKHOutput(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()
	args.IsSignAction = true
	args.Options.TrustSelf = to.Ptr(sdk.TrustSelfKnown)
	args.Outputs = []wdk.ValidCreateActionOutput{}
	args.Inputs = []wdk.ValidCreateActionInput{{
		Outpoint: wdk.OutPoint{
			TxID: unknownParentTx.ID().String(),
			Vout: 0,
		},
		UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
		InputDescription:      "provided unknown-by-storage input",
	}}

	// when:
	_, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.Error(t, err)
}

func TestCreateActionWithKnownTxIDs(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	faucetTx, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs(func(args *wdk.ValidCreateActionArgs) {
		args.Options.KnownTxids = []primitives.TXIDHexString{
			primitives.TXIDHexString(faucetTx.ID().String()),
		}
	})

	// when:
	result, err := activeStorage.CreateAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	pkgtestabilities.AssertBEEFState(t, result.InputBeef, pkgtestabilities.ExpectedBeefTransactionState{
		ID:         faucetTx.ID().String(),
		DataFormat: to.Ptr(transaction.TxIDOnly),
	})
}
