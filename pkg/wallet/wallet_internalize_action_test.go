package wallet_test

import (
	"context"
	"testing"

	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalizeActionOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error) {
			return w.InternalizeAction(ctx, args, originator)
		},
		func() sdk.InternalizeActionArgs {
			return fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
		},
	)
}

func TestWalletInternalizeActionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.InternalizeActionArgs
	}{
		"empty args": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.InternalizeActionArgs {
				return sdk.InternalizeActionArgs{}
			},
		},
		"empty transaction data": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.InternalizeActionArgs {
				args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
				args.Tx = nil
				return args
			},
		},
		"empty outputs": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.InternalizeActionArgs {
				args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
				args.Outputs = nil
				return args
			},
		},
		"invalid description (too short)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.InternalizeActionArgs {
				args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
				args.Description = "a"
				return args
			},
		},
		"invalid output protocol": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.InternalizeActionArgs {
				args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
				args.Outputs[0].Protocol = "invalid-protocol"
				return args
			},
		},
		"missing payment remittance for wallet payment protocol": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.InternalizeActionArgs {
				args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
				args.Outputs[0].PaymentRemittance = nil
				return args
			},
		},
		"missing insertion remittance for basket insertion protocol": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.InternalizeActionArgs {
				args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolBasketInsertion)
				args.Outputs[0].InsertionRemittance = nil
				return args
			},
		},
	}
	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			// and:
			aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

			// when:
			action, err := aliceWallet.InternalizeAction(t.Context(), test.args(), test.originator)

			// then:
			then.Result(action).HasError(err)

			then.Storage().HadNoInteraction()
		})
	}
}

func (s *WalletTestSuite) TestWalletInternalizeAction() {
	s.Run("wallet payment protocol", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
		internalizedTx, err := transaction.NewTransactionFromBEEF(args.Tx)
		require.NoError(t, err)

		// when:
		result, err := aliceWallet.InternalizeAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Accepted, "Result should be accepted")

		// and check db state:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(1)

		thenInternalizedAction := thenState.ActionAtIndex(0)
		thenInternalizedAction.
			WithTxID(internalizedTx.TxID().String()).
			WithSatoshis(int64(internalizedTx.Outputs[0].Satoshis)).
			WithDescription(args.Description)

		thenInternalizedAction.OutputAtIndex(0).
			WithSatoshis(internalizedTx.Outputs[0].Satoshis).
			WithLockingScript(internalizedTx.Outputs[0].LockingScript.Bytes()).
			WithOutputIndex(0).
			WithBasket(wdk.BasketNameForChange).
			WithSpendable(true)
	})

	s.Run("basket insertion protocol", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolBasketInsertion)
		internalizedTx, err := transaction.NewTransactionFromBEEF(args.Tx)
		require.NoError(t, err)

		// when:
		result, err := aliceWallet.InternalizeAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Accepted, "Result should be accepted")

		// and check db state:
		thenState := testabilities.ThenWalletState(t, aliceWallet)
		thenState.
			HasActionsCount(1)

		thenInternalizedAction := thenState.ActionAtIndex(0)
		thenInternalizedAction.
			WithTxID(internalizedTx.TxID().String()).
			WithSatoshis(0).
			WithDescription(args.Description)

		thenInternalizedAction.OutputAtIndex(0).
			WithSatoshis(internalizedTx.Outputs[0].Satoshis).
			WithLockingScript(internalizedTx.Outputs[0].LockingScript.Bytes()).
			WithOutputIndex(0).
			WithBasket(fixtures.CustomBasket).
			WithSpendable(true)
	})
}
