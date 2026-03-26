package integrationtests

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder/errfunder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/tsgenerated"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const (
	derPrefix = "Pr=="
	derSuffix = "Su=="
)

func TestInternalizeThenCreateThenProcess(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	var createdTxReference string
	var processedTxID string

	t.Run("Internalize", func(t *testing.T) {
		// given:
		args := wdk.InternalizeActionArgs{
			Tx: tsgenerated.ParentTransactionAtomicBeef(t),
			Outputs: []*wdk.InternalizeOutput{
				{
					OutputIndex: 0,
					Protocol:    wdk.WalletPaymentProtocol,
					PaymentRemittance: &wdk.WalletPayment{
						DerivationPrefix:  derPrefix,
						DerivationSuffix:  derSuffix,
						SenderIdentityKey: fixtures.AnyoneIdentityKey,
					},
				},
			},
			Labels: []primitives.StringUnder300{
				"label1", "label2",
			},
			Description:    "description",
			SeekPermission: nil,
		}

		// and:
		given.Provider().BHS().OnMerkleRootVerifyResponse(
			tsgenerated.BeefToInternalizeHeight,
			tsgenerated.BeefToInternalizeMerkleRoot,
			testabilities.BHSMerkleRootConfirmed,
		)

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)

		// when:
		resultJSON, err := json.Marshal(result)

		// then:
		require.NoError(t, err)

		require.JSONEq(t, `{
		  "accepted": true,
		  "isMerge": false,
		  "txid": "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6",
		  "satoshis": 99904
		}`, string(resultJSON))
	})

	t.Run("Create", func(t *testing.T) {
		// given:
		args := createActionArgsWithProvidedOutput()

		// when:
		result, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)

		// when:
		// then:
		require.NoError(t, err)
		require.Len(t, result.Outputs, 9)
		require.Len(t, result.Inputs, 1)

		// update:
		createdTxReference = result.Reference
	})

	t.Run("Process", func(t *testing.T) {
		// given:
		tx := tsgenerated.SignedTransaction(t)
		txID := tx.TxID().String()

		// and:
		args := wdk.ProcessActionArgs{
			IsNewTx:    true,
			IsSendWith: false,
			IsNoSend:   false,
			IsDelayed:  false,
			Reference:  to.Ptr(createdTxReference),
			TxID:       to.Ptr(primitives.TXIDHexString(txID)),
			RawTx:      tx.Bytes(),
			SendWith:   []primitives.TXIDHexString{},
		}

		// when:
		result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

		// then:
		require.NoError(t, err)

		require.Len(t, result.SendWithResults, 1)
		sendWithResult := result.SendWithResults[0]
		assert.Equal(t, txID, string(sendWithResult.TxID))
		assert.Equal(t, wdk.SendWithResultStatusUnproven, sendWithResult.Status)

		require.Len(t, result.NotDelayedResults, 1)
		reviewActionResult := result.NotDelayedResults[0]
		assert.Equal(t, txID, string(reviewActionResult.TxID))
		assert.Equal(t, wdk.ReviewActionResultStatusSuccess, reviewActionResult.Status)
		assert.Empty(t, reviewActionResult.CompetingTxs)

		// update:
		processedTxID = txID
	})

	t.Run("Next create - to check if another new transaction can be created using generated change UTXOs", func(t *testing.T) {
		// given:
		args := createActionArgsWithProvidedOutput()

		// when:
		result, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		require.Len(t, result.Inputs, 1)
		assert.Equal(t, processedTxID, result.Inputs[0].SourceTxID)
		require.Len(t, result.Outputs, 9)
	})
}

func TestCreateWithUnknownInputThenProcess(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	var createdTxReference string
	var processedTxID string

	// given:
	basketConf := wdk.DefaultBasketConfiguration()
	basketConf.NumberOfDesiredUTXOs = 31 // we need to adjust the number of generated change outputs to align with the tsgenerated.SignedTransaction
	err := activeStorage.ConfigureBasket(t.Context(), testusers.Alice.AuthID(), basketConf)
	require.NoError(t, err)

	// and:
	given.Provider().BHS().OnMerkleRootVerifyResponse(
		tsgenerated.BeefToInternalizeHeight,
		tsgenerated.BeefToInternalizeMerkleRoot,
		testabilities.BHSMerkleRootConfirmed,
	)

	t.Run("Create", func(t *testing.T) {
		// given:
		args := createActionArgsWithProvidedOutput()
		args.Inputs = []wdk.ValidCreateActionInput{
			{
				Outpoint: wdk.OutPoint{
					TxID: "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6",
					Vout: 0,
				},
				InputDescription:      "unknown-to-storage utxo",
				UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
			},
		}
		args.IsSignAction = true
		args.InputBEEF = tsgenerated.ParentTransactionAtomicBeef(t)

		// when:
		result, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		require.Len(t, result.Inputs, 1)
		require.Equal(t, wdk.ProvidedByYou, result.Inputs[0].ProvidedBy)

		// update:
		createdTxReference = result.Reference
	})

	t.Run("Process", func(t *testing.T) {
		// given:
		tx := tsgenerated.SignedTransaction(t)
		txID := tx.TxID().String()

		// and:
		args := wdk.ProcessActionArgs{
			IsNewTx:    true,
			IsSendWith: false,
			IsNoSend:   false,
			IsDelayed:  false,
			Reference:  to.Ptr(createdTxReference),
			TxID:       to.Ptr(primitives.TXIDHexString(txID)),
			RawTx:      tx.Bytes(),
			SendWith:   []primitives.TXIDHexString{},
		}

		// when:
		result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

		// then:
		require.NoError(t, err)

		require.Len(t, result.SendWithResults, 1)
		sendWithResult := result.SendWithResults[0]
		assert.Equal(t, txID, string(sendWithResult.TxID))
		assert.Equal(t, wdk.SendWithResultStatusUnproven, sendWithResult.Status)

		require.Len(t, result.NotDelayedResults, 1)
		reviewActionResult := result.NotDelayedResults[0]
		assert.Equal(t, txID, string(reviewActionResult.TxID))
		assert.Equal(t, wdk.ReviewActionResultStatusSuccess, reviewActionResult.Status)
		assert.Empty(t, reviewActionResult.CompetingTxs)

		// update:
		processedTxID = txID
	})

	t.Run("Next create - to check if another new transaction can be created using generated change UTXOs", func(t *testing.T) {
		// given:
		args := createActionArgsWithProvidedOutput()

		// when:
		result, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		require.Len(t, result.Inputs, 1)
		assert.Equal(t, processedTxID, result.Inputs[0].SourceTxID)
		require.Len(t, result.Outputs, 9)
	})
}

func TestCreateWithKnownInputThenProcess(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().
		WithRandomizer(randomizer.NewTestRandomizer()).
		GORM()

	var createdTxReference string
	var processedTxID string

	given.Provider().BHS().OnMerkleRootVerifyResponse(
		tsgenerated.BeefToInternalizeHeight,
		tsgenerated.BeefToInternalizeMerkleRoot,
		testabilities.BHSMerkleRootConfirmed,
	)

	t.Run("Internalize - this way the storage will 'know' specified UTXO", func(t *testing.T) {
		// given:
		args := wdk.InternalizeActionArgs{
			Tx: tsgenerated.ParentTransactionAtomicBeef(t),
			Outputs: []*wdk.InternalizeOutput{
				{
					OutputIndex: 0,
					Protocol:    wdk.WalletPaymentProtocol,
					PaymentRemittance: &wdk.WalletPayment{
						DerivationPrefix:  derPrefix,
						DerivationSuffix:  derSuffix,
						SenderIdentityKey: fixtures.AnyoneIdentityKey,
					},
				},
			},
			Labels: []primitives.StringUnder300{
				"label1", "label2",
			},
			Description:    "description",
			SeekPermission: nil,
		}

		// when:
		_, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
	})

	t.Run("Create", func(t *testing.T) {
		// given:
		args := createActionArgsWithProvidedOutput()
		args.Inputs = []wdk.ValidCreateActionInput{
			{
				Outpoint: wdk.OutPoint{
					TxID: "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6",
					Vout: 0,
				},
				InputDescription:      "known-to-storage utxo",
				UnlockingScriptLength: to.Ptr(primitives.PositiveInteger(108)),
			},
		}
		args.IsSignAction = true
		args.InputBEEF = tsgenerated.ParentTransactionAtomicBeef(t)

		// when:
		result, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		require.Len(t, result.Inputs, 1)
		require.Equal(t, wdk.ProvidedByYouAndStorage, result.Inputs[0].ProvidedBy)

		// update:
		createdTxReference = result.Reference
	})

	t.Run("Process", func(t *testing.T) {
		// given:
		tx := tsgenerated.SignedTransaction(t)
		txID := tx.TxID().String()

		// and:
		args := wdk.ProcessActionArgs{
			IsNewTx:    true,
			IsSendWith: false,
			IsNoSend:   false,
			IsDelayed:  false,
			Reference:  to.Ptr(createdTxReference),
			TxID:       to.Ptr(primitives.TXIDHexString(txID)),
			RawTx:      tx.Bytes(),
			SendWith:   []primitives.TXIDHexString{},
		}

		// when:
		result, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), args)

		// then:
		require.NoError(t, err)

		require.Len(t, result.SendWithResults, 1)
		sendWithResult := result.SendWithResults[0]
		assert.Equal(t, txID, string(sendWithResult.TxID))
		assert.Equal(t, wdk.SendWithResultStatusUnproven, sendWithResult.Status)

		require.Len(t, result.NotDelayedResults, 1)
		reviewActionResult := result.NotDelayedResults[0]
		assert.Equal(t, txID, string(reviewActionResult.TxID))
		assert.Equal(t, wdk.ReviewActionResultStatusSuccess, reviewActionResult.Status)
		assert.Empty(t, reviewActionResult.CompetingTxs)

		// update:
		processedTxID = txID
	})

	t.Run("Next create - to check if another new transaction can be created using generated change UTXOs", func(t *testing.T) {
		// given:
		args := createActionArgsWithProvidedOutput()

		// when:
		result, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		require.Len(t, result.Inputs, 1)
		assert.Equal(t, processedTxID, result.Inputs[0].SourceTxID)
		require.Len(t, result.Outputs, 9)
	})
}

func TestInternalizePlusTooHighCreate(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().GORM()

	t.Run("Internalize", func(t *testing.T) {
		// given:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.BasketInsertionProtocol)

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		require.True(t, result.Accepted)
	})

	t.Run("Create", func(t *testing.T) {
		// given:
		args := fixtures.DefaultValidCreateActionArgs()
		args.Outputs[0].Satoshis = 2 * fixtures.ExpectedValueToInternalize

		// when:
		_, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.ErrorIs(t, err, errfunder.ErrNotEnoughFunds)
	})
}

func TestInternalizeBasketInsertionThenCreate(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().GORM()

	t.Run("Internalize", func(t *testing.T) {
		// given:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.BasketInsertionProtocol)

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		require.True(t, result.Accepted)
	})

	t.Run("Create", func(t *testing.T) {
		// given:
		args := fixtures.DefaultValidCreateActionArgs()
		args.Outputs[0].Satoshis = 1

		// when:
		_, err := activeStorage.CreateAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.ErrorIs(t, err, errfunder.ErrNotEnoughFunds)
	})
}

func createActionArgsWithProvidedOutput() wdk.ValidCreateActionArgs {
	return wdk.ValidCreateActionArgs{
		Description: "outputBRC29",
		Inputs:      []wdk.ValidCreateActionInput{},
		Outputs: []wdk.ValidCreateActionOutput{
			{
				LockingScript:      "76a9144b0d6cbef5a813d2d12dcec1de2584b250dc96a388ac",
				Satoshis:           1000,
				OutputDescription:  "outputBRC29",
				CustomInstructions: to.Ptr(`{"derivationPrefix":"Pr==","derivationSuffix":"Su==","type":"BRC29"}`),
			},
		},
		LockTime: 0,
		Version:  1,
		Labels:   []primitives.StringUnder300{"outputbrc29"},
		Options: wdk.ValidCreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr[primitives.BooleanDefaultTrue](false),
			SendWith:               []primitives.TXIDHexString{},
			SignAndProcess:         to.Ptr(primitives.BooleanDefaultTrue(true)),
			KnownTxids:             []primitives.TXIDHexString{},
			NoSendChange:           []wdk.OutPoint{},
			RandomizeOutputs:       false,
		},
		IsSendWith:                   false,
		IsDelayed:                    false,
		IsNoSend:                     false,
		IsNewTx:                      true,
		IsRemixChange:                false,
		IsSignAction:                 false,
		IncludeAllSourceTransactions: true,
	}
}
