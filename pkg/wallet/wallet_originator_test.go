package wallet_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/require"
)

// RunOriginatorValidationErrorsTests runs the standard originator validation tests for any wallet method.
func RunOriginatorValidationErrorsTests[TArgs any, TResult any](
	t *testing.T,
	walletMethod func(wallet *wallet.Wallet, ctx context.Context, args TArgs, originator string) (TResult, error),
	argsFactory func() TArgs,
) {
	errorTestCases := map[string]struct {
		originator string
		args       func() TArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args:       argsFactory,
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args:       argsFactory,
		},
		"empty originator part": {
			originator: "part1..part3",
			args:       argsFactory,
		},
	}

	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

			// when:
			result, err := walletMethod(aliceWallet, t.Context(), test.args(), test.originator)

			// then:
			then.Result(result).HasError(err)
			require.ErrorContains(t, err, "invalid originator")
			then.Storage().HadNoInteraction()
		})
	}
}
