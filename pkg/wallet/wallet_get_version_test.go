package wallet_test

import (
	"strings"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestWalletGetVersionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       sdk.RelinquishOutputArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
		},

		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
		},

		"empty part in originator": {
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
			result, err := aliceWallet.GetVersion(t.Context(), test.args, test.originator)

			// then:
			then.Result(result).HasError(err)
			then.Storage().HadNoInteraction()
		})
	}
}
