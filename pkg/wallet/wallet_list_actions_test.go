package wallet_test

import (
	"strings"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/validate"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletListActionsArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.ListActionsArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args: func() sdk.ListActionsArgs {
				return fixtures.DefaultWalletListActionsArgs()
			},
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args: func() sdk.ListActionsArgs {
				return fixtures.DefaultWalletListActionsArgs()
			},
		},
		"invalid limit (too high)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListActionsArgs {
				args := fixtures.DefaultWalletListActionsArgs()
				args.Limit = validate.MaxPaginationLimit + 1
				return args
			},
		},
		"too long label": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListActionsArgs {
				args := fixtures.DefaultWalletListActionsArgs()
				args.Labels = []string{strings.Repeat("a", 301)}
				return args
			},
		},
		"seek permission false": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListActionsArgs {
				args := fixtures.DefaultWalletListActionsArgs()
				args.SeekPermission = to.Ptr(false)
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
			result, err := aliceWallet.ListActions(t.Context(), test.args(), test.originator)

			// then:
			then.Result(result).HasError(err)

			then.Storage().HadNoInteraction()
		})
	}
}

func (s *WalletTestSuite) TestWalletListActions() {
	s.Run("empty result when no actions exist", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletListActionsArgs()

		// when:
		result, err := aliceWallet.ListActions(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, uint32(0), result.TotalActions, "Should have zero total actions")
		assert.Empty(t, result.Actions, "Actions list should be empty")
	})

	s.Run("list actions after create action", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		createArgs := fixtures.DefaultWalletCreateActionArgs(t)
		createResult, err := aliceWallet.CreateAction(t.Context(), createArgs, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotNil(t, createResult)

		// when:
		listArgs := fixtures.DefaultWalletListActionsArgsWithIncludes()
		listArgs.Labels = createArgs.Labels
		result, err := aliceWallet.ListActions(t.Context(), listArgs, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, uint32(1), result.TotalActions, "Should have one action")
		require.Len(t, result.Actions, 1)

		// and:
		action := result.Actions[0]
		assert.NotEmpty(t, action.Txid, "Should have a transaction ID")
		assert.Equal(t, createArgs.Description, action.Description)
		assert.Equal(t, createArgs.Version, action.Version)
		assert.Equal(t, createArgs.LockTime, action.LockTime)
		assert.ElementsMatch(t, createArgs.Labels, action.Labels)
		assert.NotNil(t, action.Inputs, "Inputs should be included")
		assert.NotNil(t, action.Outputs, "Outputs should be included")
	})

	s.Run("list actions after internalize action", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
		internalizeResult, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotNil(t, internalizeResult)
		require.True(t, internalizeResult.Accepted)

		// when:
		listArgs := fixtures.DefaultWalletListActionsArgsWithIncludes()
		listArgs.Labels = internalizeArgs.Labels
		result, err := aliceWallet.ListActions(t.Context(), listArgs, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, uint32(1), result.TotalActions, "Should have one action")
		require.Len(t, result.Actions, 1)

		// and:
		action := result.Actions[0]
		assert.NotEmpty(t, action.Txid, "Should have a transaction ID")
		assert.Equal(t, internalizeArgs.Description, action.Description)
		assert.ElementsMatch(t, internalizeArgs.Labels, action.Labels)
		assert.NotNil(t, action.Inputs, "Inputs should be included")
		assert.NotNil(t, action.Outputs, "Outputs should be included")
	})
}
