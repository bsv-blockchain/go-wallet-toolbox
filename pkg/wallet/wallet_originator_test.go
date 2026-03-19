package wallet_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

// RunOriginatorValidationErrorTests runs the standard originator validation tests for any wallet method.
func RunOriginatorValidationErrorTests[TResult any](
	t *testing.T,
	walletMethod func(wallet *wallet.Wallet, ctx context.Context, originator string) (TResult, error),
) {
	errorTestCases := map[string]struct {
		originator string
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
		},
		"empty originator part": {
			originator: "a..c",
		},
	}

	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()

			aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

			// when:
			result, err := walletMethod(aliceWallet, t.Context(), test.originator)

			// then:
			then.Result(result).HasError(err)
			require.ErrorContains(t, err, "invalid originator")
			then.Storage().HadNoInteraction()
		})
	}
}
