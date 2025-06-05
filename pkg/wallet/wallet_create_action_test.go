package wallet_test

import (
	"strings"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestWalletCreateActionArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       func() sdk.CreateActionArgs
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args: func() sdk.CreateActionArgs {
				return fixtures.DefaultWalletCreateActionArgs(t)
			},
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args: func() sdk.CreateActionArgs {
				return fixtures.DefaultWalletCreateActionArgs(t)
			},
		},
		"empty args": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				return sdk.CreateActionArgs{}
			},
		},
		"invalid description (too short)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Description = "a"
				return args
			},
		},
		"invalid description (too long)": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Description = strings.Repeat("a", 2001)
				return args
			},
		},
		"invalid output locking script": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].LockingScript = "invalid-hex"
				return args
			},
		},
		"too big output satoshis": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].Satoshis = primitives.MaxSatoshis + 1
				return args
			},
		},
		"too short output description": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].OutputDescription = "a"
				return args
			},
		},
		"too long output description": {
			originator: fixtures.DefaultOriginator,
			args: func() sdk.CreateActionArgs {
				args := fixtures.DefaultWalletCreateActionArgs(t)
				args.Outputs[0].OutputDescription = strings.Repeat("a", 2001)
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
			action, err := aliceWallet.CreateAction(t.Context(), test.args(), test.originator)

			// then:
			then.Result(action).HasError(err)

			then.Storage().HadNoInteraction()
		})
	}
}
