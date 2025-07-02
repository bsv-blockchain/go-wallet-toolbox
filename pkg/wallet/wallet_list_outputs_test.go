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

func TestWalletListOutputsArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.ListOutputsArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args: func() sdk.ListOutputsArgs {
				return fixtures.DefaultWalletListOutputsArgs()
			},
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args: func() sdk.ListOutputsArgs {
				return fixtures.DefaultWalletListOutputsArgs()
			},
		},
		"invalid limit (too high)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListOutputsArgs {
				args := fixtures.DefaultWalletListOutputsArgs()
				args.Limit = 10001
				return args
			},
		},
		"invalid limit (zero)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListOutputsArgs {
				args := fixtures.DefaultWalletListOutputsArgs()
				args.Limit = 0
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
			result, err := aliceWallet.ListOutputs(t.Context(), test.args(), test.originator)

			// then:
			then.Result(result).HasError(err)

			then.Storage().HadNoInteraction()
		})
	}
}

func (s *WalletTestSuite) TestWalletListOutputs() {
	s.Run("list outputs with empty result when no outputs exist", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.Equal(t, 0, len(result.Outputs), "Should have no outputs when none exist")
		assert.Equal(t, uint32(0), result.TotalOutputs, "Total outputs should be zero")
	})

	s.Run("basic list outputs after internalize action", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.Greater(t, len(result.Outputs), 0, "Should have at least one output after internalize")
		assert.Equal(t, uint64(fixtures.ExpectedValueToInternalize), result.Outputs[0].Satoshis, "Output value should match internalized amount")
	})

	s.Run("list outputs with custom limit after internalize action", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Limit = 50

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.Greater(t, len(result.Outputs), 0, "Should have at least one output after internalize")
	})

	s.Run("list outputs with include entire transactions after internalize action", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Include = sdk.OutputIncludeEntireTransactions

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.Greater(t, len(result.Outputs), 0, "Should have at least one output after internalize")
		assert.NotNil(t, result.BEEF, "BEEF should be included when requesting entire transactions")
	})

	s.Run("list outputs with include locking scripts after internalize action", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolWalletPayment)
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Include = sdk.OutputIncludeLockingScripts

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.Greater(t, len(result.Outputs), 0, "Should have at least one output after internalize")
		assert.NotNil(t, result.Outputs[0].LockingScript, "Locking script should be included")
		assert.Greater(t, len(result.Outputs[0].LockingScript), 0, "Locking script should not be empty")
	})

	s.Run("list outputs with basket insertion protocol", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and: first internalize an action using basket insertion protocol
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgs(t, sdk.InternalizeProtocolBasketInsertion)
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and: list outputs from the custom basket
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Basket = fixtures.CustomBasket
		trueValue := true
		args.IncludeTags = &trueValue
		args.IncludeLabels = &trueValue
		args.IncludeCustomInstructions = &trueValue

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.Greater(t, len(result.Outputs), 0, "Should have at least one output in custom basket")
		assert.Equal(t, uint64(fixtures.ExpectedValueToInternalize), result.Outputs[0].Satoshis, "Output value should match internalized amount")

		// and:
		assert.NotNil(t, result.Outputs[0].Tags, "Tags should be included")
		assert.Greater(t, len(result.Outputs[0].Tags), 0, "Should have tags")
		assert.Contains(t, result.Outputs[0].Tags, "tag1", "Should contain expected tag")
		assert.Contains(t, result.Outputs[0].Tags, "tag2", "Should contain expected tag")
		assert.NotEmpty(t, result.Outputs[0].CustomInstructions, "Custom instructions should be included")
	})

	
}
