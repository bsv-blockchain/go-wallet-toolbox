package wallet_test

import (
	"context"
	"strings"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestGetNetworkOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.GetNetworkResult, error) {
			return w.GetNetwork(ctx, nil, originator)
		},
	)
}

func (s *WalletTestSuite) TestWalletGetNetwork() {
	successTestCases := map[string]struct {
		args            any
		originator      string
		expectedNetwork sdk.Network
	}{
		"default originator returns testnet": {
			args:            nil,
			originator:      fixtures.DefaultOriginator,
			expectedNetwork: sdk.Network(defs.NetworkTestnet),
		},
		"simple originator returns testnet": {
			args:            nil,
			originator:      "testoriginator",
			expectedNetwork: sdk.Network(defs.NetworkTestnet),
		},
		"multi-part originator returns testnet": {
			args:            nil,
			originator:      "subdomain.example.com",
			expectedNetwork: sdk.Network(defs.NetworkTestnet),
		},
		"max single label length (63) across multiple parts": {
			originator:      strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63),
			args:            nil,
			expectedNetwork: sdk.Network(defs.NetworkTestnet),
		},
		"max total length (250 chars) should pass": {
			originator:      strings.Repeat("a", 250),
			args:            nil,
			expectedNetwork: sdk.Network(defs.NetworkTestnet),
		},
		"case-insensitive originator (if normalization is supported)": {
			originator:      strings.ToUpper(fixtures.DefaultOriginator),
			args:            nil,
			expectedNetwork: sdk.Network(defs.NetworkTestnet),
		},
	}

	for name, test := range successTestCases {
		s.Run(name, func() {
			t := s.T()

			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()
			aliceWallet := given.AliceWalletWithStorage(s.StorageType)

			// when:
			result, err := aliceWallet.GetNetwork(t.Context(), test.args, test.originator)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, test.expectedNetwork, result.Network, "Should return the correct network")
		})
	}
}
