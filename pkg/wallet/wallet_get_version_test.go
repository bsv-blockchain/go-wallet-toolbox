package wallet_test

import (
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func TestWalletGetVersionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
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
			result, err := aliceWallet.GetVersion(t.Context(), nil, test.originator)

			// then:
			then.Result(result).HasError(err)
			then.Storage().HadNoInteraction()
		})
	}
}

func TestWalletGetVersion(t *testing.T) {
	// given:
	given, then, cleanup := testabilities.New(t)
	defer cleanup()
	aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

	// when:
	result, err := aliceWallet.GetVersion(t.Context(), nil, "alice.wallet")

	// then:
	require.NoError(t, err)
	require.Equal(t, defs.Version, result.Version)
	then.Storage().HadNoInteraction()
}
