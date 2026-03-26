package wallet_test

import (
	"context"
	"strings"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestListActionsOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.ListActionsResult, error) {
			args := sdk.ListActionsArgs{}
			return w.ListActions(ctx, args, originator)
		},
	)
}

func TestWalletListActionsArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.ListActionsArgs
	}{
		"invalid limit (too high)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.ListActionsArgs {
				args := fixtures.DefaultWalletListActionsArgs()
				args.Limit = to.Ptr[uint32](validate.MaxPaginationLimit + 1)
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
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			// and:
			aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

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
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args := fixtures.DefaultWalletListActionsArgs()

		// when:
		result, err := aliceWallet.ListActions(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, uint32(0), result.TotalActions, "Should have zero total actions")
		assert.Empty(t, result.Actions, "Actions list should be empty")
	})
}
