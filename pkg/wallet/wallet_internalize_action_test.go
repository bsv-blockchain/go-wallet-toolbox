package wallet_test

import (
	"strings"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletInternalizeActionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.InternalizeActionArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args: func() sdk.InternalizeActionArgs {
				return fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
			},
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args: func() sdk.InternalizeActionArgs {
				return fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
			},
		},
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
			given, then := testabilities.New(t)

			// and:
			aliceWallet, cleanup := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)
			defer cleanup()

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
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)

		// when:
		result, err := aliceWallet.InternalizeAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Accepted, "Result should be accepted")

		// when: verify that ListActions shows the internalized action
		listArgs := fixtures.DefaultWalletListActionsArgsWithIncludes()
		listArgs.Labels = args.Labels
		listResult, listErr := aliceWallet.ListActions(t.Context(), listArgs, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, listErr)
		require.NotNil(t, listResult)
		assert.Equal(t, uint32(1), listResult.TotalActions, "Should have one action after internalize")
		require.Len(t, listResult.Actions, 1)
	})

	s.Run("basket insertion protocol", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolBasketInsertion)

		// when:
		result, err := aliceWallet.InternalizeAction(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Accepted, "Result should be accepted")
	})
}
