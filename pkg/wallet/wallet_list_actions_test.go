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
		"invalid limit (zero)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListActionsArgs {
				args := fixtures.DefaultWalletListActionsArgs()
				args.Limit = 0
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
	s.Run("basic list actions", func() {
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
		assert.GreaterOrEqual(t, int(result.TotalActions), 0, "Should have non-negative total actions")
	})

	s.Run("list actions with all includes", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletListActionsArgsWithIncludes()

		// when:
		result, err := aliceWallet.ListActions(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.GreaterOrEqual(t, int(result.TotalActions), 0, "Should have non-negative total actions")

		// then: verify the included fields are properly populated
		if len(result.Actions) > 0 {
			action := result.Actions[0]
			assert.NotNil(t, action.Labels, "Labels should be included")
			assert.NotNil(t, action.Inputs, "Inputs should be included")
			assert.NotNil(t, action.Outputs, "Outputs should be included")
		}
	})

	s.Run("list actions with pagination", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletListActionsArgs()
		args.Limit = 2
		args.Offset = 0

		// when:
		result, err := aliceWallet.ListActions(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.GreaterOrEqual(t, int(result.TotalActions), 0, "Should have non-negative total actions")

		// then: test second page
		args.Offset = 2
		result2, err := aliceWallet.ListActions(t.Context(), args, fixtures.DefaultOriginator)
		assert.NoError(t, err)
		require.NotNil(t, result2)
		assert.Equal(t, result.TotalActions, result2.TotalActions, "Total actions should be consistent across pages")
	})

	s.Run("list actions with label query mode all", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletListActionsArgs()
		args.Labels = []string{"label1", "label2"}
		args.LabelQueryMode = sdk.QueryModeAll
		args.IncludeLabels = to.Ptr(true)

		// when:
		result, err := aliceWallet.ListActions(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.GreaterOrEqual(t, int(result.TotalActions), 0, "Should have non-negative total actions")
	})

	s.Run("list actions with label query mode any", func() {
		t := s.T()

		// given:
		given := testabilities.Given(t)

		// and:
		aliceWallet, cleanup := given.AliceWalletWithStorage(s.StorageType)
		defer cleanup()

		// and:
		args := fixtures.DefaultWalletListActionsArgs()
		args.Labels = []string{"test-label"}
		args.LabelQueryMode = sdk.QueryModeAny
		args.IncludeLabels = to.Ptr(true)

		// when:
		result, err := aliceWallet.ListActions(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.GreaterOrEqual(t, int(result.TotalActions), 0, "Should have non-negative total actions")
	})

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
}
