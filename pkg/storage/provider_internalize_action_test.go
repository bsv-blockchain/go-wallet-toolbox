package storage_test

import (
	"context"
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
	assert.Equal(t, primitives.SatoshiValue(fixtures.ExpectedValueToInternalize), result.Satoshis)
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
	assert.Equal(t, primitives.SatoshiValue(0), result.Satoshis)
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

func TestInternalizeActionForStoredTransaction(t *testing.T) {
	t.Skip("This test is not implemented yet, see")
	// TODO
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// and:
	ownedTxSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100_000)
	ownedAtomicBeef, _ := ownedTxSpec.TX().AtomicBEEF(false)

	// and:
	args := fixtures.DefaultInternalizeActionArgs(t, wdk.WalletPaymentProtocol)
	args.Tx = ownedAtomicBeef

	// then:
	require.Panics(t, func() {
		// when:
		_, _ = activeStorage.InternalizeAction(
			context.Background(),
			testusers.Alice.AuthID(),
			args,
		)
	})

}
