package storage_test

import (
	"context"
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestInternalizeAction_UpdateKnownTxAsMined_HappyPath(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	provider := given.Provider()
	activeStorage := provider.GORM()
	whatsOnChain := provider.WhatsOnChain()

	// and:
	tx := whatsOnChain.MinedTransaction().Tx()
	txID := tx.TxID()

	root, err := tx.MerklePath.ComputeRoot(txID)
	require.NoError(t, err)
	require.NotNil(t, root)

	atomicBEEF, err := tx.AtomicBEEF(false)
	require.NoError(t, err)
	require.NotNil(t, atomicBEEF)

	// and:
	whatsOnChain.WillRespondWithMerkleRoot(root.String())

	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
	args.Tx = atomicBEEF

	// and:
	expectedResult := &wdk.InternalizeActionResult{
		Accepted: true,
		IsMerge:  false,
		TxID:     txID.String(),
		Satoshis: 2324,
	}

	// when:
	actualResult, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)
	require.Equal(t, expectedResult, actualResult)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(txID.String()).WithBlockHash(to.Ptr(pkgtestabilities.TestBlockHash))
}

func TestInternalizeActionNilAuth(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.InternalizeAction(t.Context(), wdk.AuthID{UserID: nil}, fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol))

	// then:
	require.Error(t, err)
}

func TestInternalizeActionWalletPaymentHappyPath(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)

	// when:
	result, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)

	assert.True(t, result.Accepted)
	assert.False(t, result.IsMerge)
	assert.Equal(t, int64(fixtures.ExpectedValueToInternalize), result.Satoshis)
	assert.Equal(t, "03895fb984362a4196bc9931629318fcbb2aeba7c6293638119ea653fa31d119", result.TxID)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(result.TxID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.
				Count(1).
				Note("internalizeAction", to.Ptr(testusers.Alice.ID), nil)
		})

	thenDBState.AllOutputs(testusers.Alice).WithCountHavingTxID(1)
}

func TestInternalizeActionBasketInsertionHappyPath(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	args := fixtures.DefaultInternalizeActionArgs(t, wdk.BasketInsertionProtocol)

	// when:
	result, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)

	assert.True(t, result.Accepted)
	assert.False(t, result.IsMerge)
	assert.Equal(t, int64(0), result.Satoshis)
	assert.Equal(t, "03895fb984362a4196bc9931629318fcbb2aeba7c6293638119ea653fa31d119", result.TxID)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(result.TxID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined).
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.
				Count(1).
				Note("internalizeAction", to.Ptr(testusers.Alice.ID), nil)
		})

	thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCount(0)
	thenDBState.Outputs(testusers.Alice, fixtures.CustomBasket).WithCountHavingTxID(1)
}

func TestInternalizeActionErrorCases(t *testing.T) {
	tests := map[string]struct {
		modifier func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs
	}{
		"Wrong beef": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Tx = []byte{0, 1, 2, 3}
				return args
			},
		},
		"Output index out of range of provided tx": {
			modifier: func(args wdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
				args.Outputs[0].OutputIndex = fixtures.ExpectedValueToInternalize
				return args
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			// given:
			activeStorage := given.Provider().GORM()

			// and:
			args := test.modifier(fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol))

			// when:
			_, err := activeStorage.InternalizeAction(
				t.Context(),
				testusers.Alice.AuthID(),
				args,
			)

			// then:
			require.Error(t, err)

			// and db state:
			thenDBState := testabilities.ThenDBState(t, activeStorage)
			thenDBState.AllOutputs(testusers.Alice).WithCount(0)
		})
	}
}

func TestInternalizeActionForAlreadyStoredTransaction(t *testing.T) {
	t.Run("the same output", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		const alreadyOwnedSatoshis = 100_000
		ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(alreadyOwnedSatoshis)

		// and:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
		args.Tx = ownedTxSpec.AtomicBEEF().Bytes()

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		assert.Equal(t, ownedTxSpec.ID().String(), result.TxID)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(result.TxID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined)

		thenDBState.AllOutputs(testusers.Alice).WithCount(1)
		thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCountHavingTxID(1)
	})

	t.Run("two outputs - two basket insertions", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		transactionSpec := testvectors.GivenTX().
			WithInput(20_001).
			WithP2PKHOutput(10_000).
			WithP2PKHOutput(10_000)

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			wdk.InternalizeActionArgs{
				Tx: transactionSpec.AtomicBEEF().Bytes(),
				Outputs: []*wdk.InternalizeOutput{
					{
						OutputIndex: 0,
						Protocol:    wdk.BasketInsertionProtocol,
						InsertionRemittance: &wdk.BasketInsertion{
							Basket: fixtures.CustomBasket,
							Tags:   []primitives.StringUnder300{"custom_tag", "tag_for_first_output"},
						},
					},
				},
				Description: "first internalize",
			},
		)

		// then:
		require.NoError(t, err)
		assert.True(t, result.Accepted)
		assert.False(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)

		// when:
		result, err = activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			wdk.InternalizeActionArgs{
				Tx: transactionSpec.AtomicBEEF().Bytes(),
				Outputs: []*wdk.InternalizeOutput{
					{
						OutputIndex: 1,
						Protocol:    wdk.BasketInsertionProtocol,
						InsertionRemittance: &wdk.BasketInsertion{
							Basket: fixtures.CustomBasket,
							Tags:   []primitives.StringUnder300{"custom_tag", "tag_for_second_output"},
						},
					},
				},
				Description: "second internalize",
			},
		)

		// then:
		require.NoError(t, err)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(result.TxID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined)

		thenDBState.AllOutputs(testusers.Alice).WithCount(2)
		thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCount(0)
		thenDBState.Outputs(testusers.Alice, fixtures.CustomBasket).
			WithCountHavingTxID(2).
			WithCountHavingTags(2, "custom_tag").
			WithCountHavingTags(1, "tag_for_first_output").
			WithCountHavingTags(1, "tag_for_second_output")
	})

	t.Run("switch from change output to custom basket", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		const alreadyOwnedSatoshis = 100_000
		ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(alreadyOwnedSatoshis)

		// and:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
		args.Tx = ownedTxSpec.AtomicBEEF().Bytes()
		args.Outputs[0].Protocol = wdk.BasketInsertionProtocol
		args.Outputs[0].InsertionRemittance = &wdk.BasketInsertion{
			Basket: fixtures.CustomBasket,
			Tags:   []primitives.StringUnder300{"custom_tag", "tag_for_first_output"},
		}

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		assert.Equal(t, ownedTxSpec.ID().String(), result.TxID)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(-alreadyOwnedSatoshis), result.Satoshis)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(result.TxID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined)

		thenDBState.AllOutputs(testusers.Alice).WithCount(1)
		thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCount(0)
		thenDBState.Outputs(testusers.Alice, fixtures.CustomBasket).
			WithCountHavingTxID(1).
			WithCountHavingTags(1, "custom_tag", "tag_for_first_output")
	})

	t.Run("switch from custom basket to change", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		internalizeArgs, _ := given.Action(activeStorage).PreInternalized()
		walletPaymentOutput := *internalizeArgs.Outputs[0]

		internalizeArgs.Description = "first internalize"
		internalizeArgs.Outputs = []*wdk.InternalizeOutput{
			{
				OutputIndex: 0,
				Protocol:    wdk.BasketInsertionProtocol,
				InsertionRemittance: &wdk.BasketInsertion{
					Basket: "custom_basket",
					Tags:   []primitives.StringUnder300{"custom_tag"},
				},
			},
		}

		// when:
		result, err := activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			*internalizeArgs,
		)

		// then:
		require.NoError(t, err)
		assert.True(t, result.Accepted)
		assert.False(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)

		// when:
		internalizeArgs.Description = "second internalize"
		internalizeArgs.Outputs = []*wdk.InternalizeOutput{&walletPaymentOutput}
		result, err = activeStorage.InternalizeAction(
			t.Context(),
			testusers.Alice.AuthID(),
			*internalizeArgs,
		)

		// then:
		require.NoError(t, err)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(fixtures.DefaultCreateActionOutputSatoshis), result.Satoshis)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasKnownTX(result.TxID).
			NotMined().
			WithStatus(wdk.ProvenTxStatusUnmined)

		thenDBState.AllOutputs(testusers.Alice).WithCount(1)
		thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).WithCountHavingTxID(1)
		thenDBState.Outputs(testusers.Alice, fixtures.CustomBasket).WithCount(0)
	})

	t.Run("add label during withMerge internalize", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		const (
			alreadyOwnedSatoshis = 100_000
			initialLabel         = "initial_label"
			labelToAdd           = "label_for_merge"
		)
		ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).
			TopUp(alreadyOwnedSatoshis, pkgtestabilities.WithLabelsTopUp(initialLabel))

		// and:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
		args.Tx = ownedTxSpec.AtomicBEEF().Bytes()
		args.Labels = []primitives.StringUnder300{labelToAdd}

		// when:
		_, err := activeStorage.InternalizeAction(
			context.Background(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)

		// and db state:
		thenDBState := testabilities.ThenDBState(t, activeStorage)
		thenDBState.HasUserTransactionByReference(testusers.Alice, fixtures.FaucetReference(ownedTxSpec.ID().String())).
			WithLabels(initialLabel, labelToAdd)
	})
}

func TestInternalizeTheSameTxByDifferentUsers(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	transactionSpec := testvectors.GivenTX().
		WithInput(20_001).
		WithP2PKHOutput(10_000).
		WithP2PKHOutput(10_000)

	// when:
	result, err := activeStorage.InternalizeAction(
		t.Context(),
		testusers.Alice.AuthID(),
		wdk.InternalizeActionArgs{
			Tx: transactionSpec.AtomicBEEF().Bytes(),
			Outputs: []*wdk.InternalizeOutput{
				{
					OutputIndex: 0,
					Protocol:    wdk.BasketInsertionProtocol,
					InsertionRemittance: &wdk.BasketInsertion{
						Basket: fixtures.CustomBasket,
					},
				},
			},
			Description: "first internalize",
		},
	)

	// then:
	require.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.False(t, result.IsMerge)
	assert.Equal(t, int64(0), result.Satoshis)

	// when:
	result, err = activeStorage.InternalizeAction(
		t.Context(),
		testusers.Bob.AuthID(), // NOTE: This is a different user
		wdk.InternalizeActionArgs{
			Tx: transactionSpec.AtomicBEEF().Bytes(),
			Outputs: []*wdk.InternalizeOutput{
				{
					OutputIndex: 1,
					Protocol:    wdk.BasketInsertionProtocol,
					InsertionRemittance: &wdk.BasketInsertion{
						Basket: fixtures.CustomBasket,
					},
				},
			},
			Description: "second internalize",
		},
	)

	// then:
	require.NoError(t, err)
	assert.True(t, result.Accepted)
	assert.False(t, result.IsMerge)
	assert.Equal(t, int64(0), result.Satoshis)

	// and db state:
	thenDBState := testabilities.ThenDBState(t, activeStorage)
	thenDBState.HasKnownTX(result.TxID).
		NotMined().
		WithStatus(wdk.ProvenTxStatusUnmined)

	thenDBState.AllOutputs(testusers.Alice).WithCount(1)
	thenDBState.AllOutputs(testusers.Bob).WithCount(1)
}
