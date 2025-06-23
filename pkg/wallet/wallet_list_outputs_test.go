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
	s.Run("basic list outputs", func() {
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
	})

	s.Run("list outputs with custom limit", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Limit = 50

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
	})

	s.Run("list outputs with include entire transactions", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Include = sdk.OutputIncludeEntireTransactions

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
	})

	s.Run("list outputs with include locking scripts", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletListOutputsArgs()
		args.Include = sdk.OutputIncludeLockingScripts

		// when:
		result, err := aliceWallet.ListOutputs(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotNil(t, result.Outputs, "Outputs should not be nil")
	})
}
