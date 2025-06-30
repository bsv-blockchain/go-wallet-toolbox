package storage_test

import (
	"context"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalizeActionNilAuth(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// when:
	_, err := activeStorage.InternalizeAction(context.Background(), wdk.AuthID{UserID: nil}, fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol))

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
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)

	assert.Equal(t, true, result.Accepted)
	assert.Equal(t, false, result.IsMerge)
	assert.Equal(t, int64(fixtures.ExpectedValueToInternalize), result.Satoshis)
	assert.Equal(t, "03895fb984362a4196bc9931629318fcbb2aeba7c6293638119ea653fa31d119", result.TxID)
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
		context.Background(),
		testusers.Alice.AuthID(),
		args,
	)

	// then:
	require.NoError(t, err)

	assert.Equal(t, true, result.Accepted)
	assert.Equal(t, false, result.IsMerge)
	assert.Equal(t, int64(0), result.Satoshis)
	assert.Equal(t, "03895fb984362a4196bc9931629318fcbb2aeba7c6293638119ea653fa31d119", result.TxID)
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
				context.Background(),
				testusers.Alice.AuthID(),
				args,
			)

			// then:
			require.Error(t, err)
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
		ownedAtomicBeef, _ := ownedTxSpec.TX().AtomicBEEF(false)

		// and:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
		args.Tx = ownedAtomicBeef

		// when:
		result, err := activeStorage.InternalizeAction(
			context.Background(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		assert.Equal(t, ownedTxSpec.ID(), result.TxID)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)
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
		beefBytes, err := transactionSpec.TX().AtomicBEEF(false)
		require.NoError(t, err)

		// when:
		result, err := activeStorage.InternalizeAction(
			context.Background(),
			testusers.Alice.AuthID(),
			wdk.InternalizeActionArgs{
				Tx: beefBytes,
				Outputs: []*wdk.InternalizeOutput{
					{
						OutputIndex: 0,
						Protocol:    wdk.BasketInsertionProtocol,
						InsertionRemittance: &wdk.BasketInsertion{
							Basket: "custom_basket",
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
			context.Background(),
			testusers.Alice.AuthID(),
			wdk.InternalizeActionArgs{
				Tx: beefBytes,
				Outputs: []*wdk.InternalizeOutput{
					{
						OutputIndex: 1,
						Protocol:    wdk.BasketInsertionProtocol,
						InsertionRemittance: &wdk.BasketInsertion{
							Basket: "custom_basket",
							Tags:   []primitives.StringUnder300{"custom_tag", "tag_for_second_output"},
						},
					},
				},
				Description: "first internalize",
			},
		)

		// then:
		require.NoError(t, err)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(0), result.Satoshis)
	})

	t.Run("two outputs - switch from change to custom basket", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()

		// and:
		const alreadyOwnedSatoshis = 100_000
		ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(alreadyOwnedSatoshis)
		ownedAtomicBeef, _ := ownedTxSpec.TX().AtomicBEEF(false)

		// and:
		args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
		args.Tx = ownedAtomicBeef
		args.Outputs[0].Protocol = wdk.BasketInsertionProtocol
		args.Outputs[0].InsertionRemittance = &wdk.BasketInsertion{
			Basket: "custom_basket",
			Tags:   []primitives.StringUnder300{"custom_tag", "tag_for_first_output"},
		}

		// when:
		result, err := activeStorage.InternalizeAction(
			context.Background(),
			testusers.Alice.AuthID(),
			args,
		)

		// then:
		require.NoError(t, err)
		assert.Equal(t, ownedTxSpec.ID(), result.TxID)
		assert.True(t, result.Accepted)
		assert.True(t, result.IsMerge)
		assert.Equal(t, int64(-alreadyOwnedSatoshis), result.Satoshis)
	})
}
