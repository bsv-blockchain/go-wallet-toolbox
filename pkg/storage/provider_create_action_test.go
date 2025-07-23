package storage_test

import (
	"encoding/hex"
	"slices"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder/errfunder"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

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
	assert.Equal(t, 24, len(result.DerivationPrefix))
	assert.Equal(t, 16, len(result.Reference))
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Equal(t, 32, len(result.Outputs))
	assert.Equal(t, 31, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(57_998), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))
	assert.Equal(t, "0200beef0000", hex.EncodeToString(result.InputBeef))
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

	require.Equal(t, 1, len(result.Inputs))
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
	assert.Equal(t, 24, len(*input.DerivationPrefix))
	require.NotEmpty(t, input.DerivationSuffix)
	assert.Equal(t, 24, len(*input.DerivationSuffix))

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
		lastNoSendChangeVout  = 30
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
	given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)

	// and:
	args := fixtures.DefaultValidCreateActionArgs()

	// when:
	result, err := activeStorage.CreateAction(t.Context(), testusers.Alice.AuthID(), args)

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
			t.Log("Random shuffle works! Found commission outputs at different vouts")
			return
		}
	}

	t.Error("Expected commission output to be shuffled, but it was not")
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
	require.ErrorIs(t, err, errfunder.NotEnoughFunds)
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
	assert.Equal(t, 24, len(result.DerivationPrefix))
	assert.Equal(t, 16, len(result.Reference))
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Equal(t, 31, len(result.Outputs))
	assert.Equal(t, 31, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(99998), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))
	assert.Equal(t, "0200beef0001021b4dc343ecd37c7707f5c7194f0f40788a62be3264e80b6433612273d4b4bbb2", hex.EncodeToString(result.InputBeef))

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p *wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	require.Equal(t, 1, len(result.Inputs))
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
	assert.Equal(t, 24, len(*input.DerivationPrefix))
	require.NotEmpty(t, input.DerivationSuffix)
	assert.Equal(t, 24, len(*input.DerivationSuffix))
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
	assert.Equal(t, 24, len(result.DerivationPrefix))
	assert.Equal(t, 16, len(result.Reference))
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Equal(t, 32, len(result.Outputs))
	assert.Equal(t, 32, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(99998), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))
	assert.Equal(t, "0200beef01fde80301010000f6282a580ebf0cebd3edbb4ac129d2d7f8a1b337ab642f70377f3d9040eca1d102010001000000012e3f4683e173b40a20527fe5719633ba070df649983614886e90e45aecf2ac56000000006b483045022100c7ddc5159fc630d28f4beeeafa73bc8d32f25b01909732d8d44b9cdbbc85888502206a0a6269bc47c633441a7b5aff120fd0760024badd660f24f713889c0ee70ecb4121034d2d6d23fbcb6eefe3e80c47044e36797dcb80d0ac5e96e732ef03c3c550a116ffffffff01a2860100000000001976a91494677c56fa2968644c90a517214338b4139899ce88ac00000000000100000001f6282a580ebf0cebd3edbb4ac129d2d7f8a1b337ab642f70377f3d9040eca1d1000000006a4730440220291e6769c2383c82fd3c06de833589d9401dbb55838bdc02a76d8d7a98d3cac302207ad2de40877eab59981f2d46dba1cdefd40846db840ae24094eb07688b3e4ee64121034d2d6d23fbcb6eefe3e80c47044e36797dcb80d0ac5e96e732ef03c3c550a116ffffffff01a0860100000000001976a9143cf53c49c322d9d811728182939aee2dca087f9888ac00000000", hex.EncodeToString(result.InputBeef))

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p *wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	require.Equal(t, 1, len(result.Inputs))
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
	assert.Equal(t, 24, len(result.DerivationPrefix))
	assert.Equal(t, 16, len(result.Reference))
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Equal(t, 33, len(result.Outputs))
	assert.Equal(t, 32, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(57998), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))
	assert.Equal(t, "0200beef01fde80301010000f6282a580ebf0cebd3edbb4ac129d2d7f8a1b337ab642f70377f3d9040eca1d102010001000000012e3f4683e173b40a20527fe5719633ba070df649983614886e90e45aecf2ac56000000006b483045022100c7ddc5159fc630d28f4beeeafa73bc8d32f25b01909732d8d44b9cdbbc85888502206a0a6269bc47c633441a7b5aff120fd0760024badd660f24f713889c0ee70ecb4121034d2d6d23fbcb6eefe3e80c47044e36797dcb80d0ac5e96e732ef03c3c550a116ffffffff01a2860100000000001976a91494677c56fa2968644c90a517214338b4139899ce88ac00000000000100000001f6282a580ebf0cebd3edbb4ac129d2d7f8a1b337ab642f70377f3d9040eca1d1000000006a4730440220291e6769c2383c82fd3c06de833589d9401dbb55838bdc02a76d8d7a98d3cac302207ad2de40877eab59981f2d46dba1cdefd40846db840ae24094eb07688b3e4ee64121034d2d6d23fbcb6eefe3e80c47044e36797dcb80d0ac5e96e732ef03c3c550a116ffffffff01a0860100000000001976a9143cf53c49c322d9d811728182939aee2dca087f9888ac00000000", hex.EncodeToString(result.InputBeef))

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p *wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	require.Equal(t, 1, len(result.Inputs))
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
	assert.Equal(t, 24, len(result.DerivationPrefix))
	assert.Equal(t, 16, len(result.Reference))
	assert.Equal(t, args.Version, result.Version)
	assert.Equal(t, args.LockTime, result.LockTime)
	assert.Equal(t, 9, len(result.Outputs))
	assert.Equal(t, 8, testutils.CountOutputsWithCondition(t, result.Outputs, testutils.ProvidedByStorageCondition))
	assert.Equal(t, primitives.SatoshiValue(7999), testutils.SumOutputsWithCondition(t, result.Outputs, testutils.SatoshiValue, testutils.ProvidedByStorageCondition))
	assert.Equal(t, "0200beef01fde80301010000afc74b81ac854adcf0d8f6c00dc0a472091a849ab6a6eba05e56a2081cfad85d02010001000000012e3f4683e173b40a20527fe5719633ba070df649983614886e90e45aecf2ac56000000006b483045022100960e5206bd3bad3df969139151f80c22a1d95bba33b4fa2624bee729a2270ad802207f97598bbe1bd6957190064eefbee29cb71400b708d8e3583b0e6284cc189ebf4121034d2d6d23fbcb6eefe3e80c47044e36797dcb80d0ac5e96e732ef03c3c550a116ffffffff01aa610000000000001976a91494677c56fa2968644c90a517214338b4139899ce88ac00000000000100000001afc74b81ac854adcf0d8f6c00dc0a472091a849ab6a6eba05e56a2081cfad85d000000006a47304402207988fdfa676f7be346e97aab0a646445efcc8ace7ebca9ff56d6177cddfde6af02201aa2a53b255aab78d270237cc8585783f783382831b7573dc29f982e53ca1b404121034d2d6d23fbcb6eefe3e80c47044e36797dcb80d0ac5e96e732ef03c3c550a116ffffffff01a8610000000000001976a9143cf53c49c322d9d811728182939aee2dca087f9888ac00000000", hex.EncodeToString(result.InputBeef))

	testutils.ForEveryOutput(t, result.Outputs, testutils.ProvidedByStorageCondition, func(p *wdk.StorageCreateTransactionSdkOutput) {
		assert.Equal(t, "change", p.Purpose)
	})

	require.Equal(t, 2, len(result.Inputs))
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
