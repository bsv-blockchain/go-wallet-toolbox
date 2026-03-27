package wallet_test

import (
	"context"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestListOutputsOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.ListOutputsResult, error) {
			args := fixtures.DefaultWalletListOutputsArgs()
			return w.ListOutputs(ctx, args, originator)
		},
	)
}

func TestWalletListOutputsArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.ListOutputsArgs
	}{
		"invalid limit (too high)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListOutputsArgs {
				args := fixtures.DefaultWalletListOutputsArgs()
				args.Limit = to.Ptr[uint32](10001)
				return args
			},
		},
		"invalid limit (zero)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListOutputsArgs {
				args := fixtures.DefaultWalletListOutputsArgs()
				args.Limit = to.Ptr[uint32](0)
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
			result, err := aliceWallet.ListOutputs(t.Context(), test.args(), test.originator)

			// then:
			then.Result(result).HasError(err)

			then.Storage().HadNoInteraction()
		})
	}
}

const shouldHaveAtLeastOneOutputMsg = "Should have at least one output after internalize"

func (s *WalletTestSuite) TestWalletListOutputs() {
	s.Run("list outputs with empty result when no outputs exist", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.Empty(t, result.Outputs, "Should have no outputs when none exist")
		assert.Equal(t, uint32(0), result.TotalOutputs, "Total outputs should be zero")
	})

	s.Run("basic list outputs after internalize action", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, shouldHaveAtLeastOneOutputMsg)
		assert.Equal(t, uint64(fixtures.ExpectedValueToInternalize), result.Outputs[0].Satoshis, "Output value should match internalized amount")
	})

	s.Run("list outputs with custom limit after internalize action", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Limit = to.Ptr[uint32](50)

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, shouldHaveAtLeastOneOutputMsg)
	})

	s.Run("list outputs with include entire transactions after internalize action", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Include = sdk.OutputIncludeEntireTransactions

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, shouldHaveAtLeastOneOutputMsg)
		assert.NotNil(t, result.BEEF, "BEEF should be included when requesting entire transactions")
	})

	s.Run("list outputs with include locking scripts after internalize action", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		internalizeArgs := fixtures.DefaultWalletInternalizeActionArgsMatchingBRC29(t, sdk.InternalizeProtocolWalletPayment, testusers.Alice.KeyDeriver(t))
		_, err := aliceWallet.InternalizeAction(t.Context(), internalizeArgs, fixtures.DefaultOriginator)
		require.NoError(t, err, "Failed to internalize action for test setup")

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Include = sdk.OutputIncludeLockingScripts

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, shouldHaveAtLeastOneOutputMsg)
		assert.NotNil(t, result.Outputs[0].LockingScript, "Locking script should be included")
		assert.NotEmpty(t, result.Outputs[0].LockingScript, "Locking script should not be empty")
	})

	s.Run("list outputs with basket insertion protocol", func() {
		t := s.T()

		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

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
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
		assert.NotEmpty(t, result.Outputs, "Should have at least one output in custom basket")
		assert.Equal(t, uint64(fixtures.ExpectedValueToInternalize), result.Outputs[0].Satoshis, "Output value should match internalized amount")

		// and:
		assert.NotNil(t, result.Outputs[0].Tags, "Tags should be included")
		assert.NotEmpty(t, result.Outputs[0].Tags, "Should have tags")
		assert.Contains(t, result.Outputs[0].Tags, "tag1", "Should contain expected tag")
		assert.Contains(t, result.Outputs[0].Tags, "tag2", "Should contain expected tag")
		assert.NotEmpty(t, result.Outputs[0].CustomInstructions, "Custom instructions should be included")
	})
}
