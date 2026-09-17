package wallet_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-sdk/wallet/serializer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestGetNetworkOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(
		t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.GetNetworkResult, error) {
			return w.GetNetwork(ctx, nil, originator)
		},
	)
}

func TestGetNetworkBRC100Values(t *testing.T) {
	tests := []struct {
		network  defs.BSVNetwork
		expected string
	}{
		{network: defs.NetworkMainnet, expected: "mainnet"},
		{network: defs.NetworkTestnet, expected: "testnet"},
		{network: defs.NetworkTTN, expected: "testnet"},
		{network: defs.NetworkTSTN, expected: "testnet"},
	}

	for _, test := range tests {
		t.Run(string(test.network), func(t *testing.T) {
			given, cleanup := testabilities.Given(t)
			defer cleanup()
			w := given.Wallet().WithNetwork(test.network).WithSQLiteStorage().ForUser(testusers.Alice)

			result, err := w.GetNetwork(t.Context(), nil, fixtures.DefaultOriginator)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, test.expected, string(result.Network))

			wire, err := json.Marshal(result)
			require.NoError(t, err)
			assert.JSONEq(t, `{"network":"`+test.expected+`"}`, string(wire))

			binaryWire, err := serializer.SerializeGetNetworkResult(result)
			require.NoError(t, err)
			decoded, err := serializer.DeserializeGetNetworkResult(binaryWire)
			require.NoError(t, err)
			assert.Equal(t, test.expected, string(decoded.Network))
		})
	}
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
			expectedNetwork: sdk.NetworkTestnet,
		},
		"simple originator returns testnet": {
			args:            nil,
			originator:      "testoriginator",
			expectedNetwork: sdk.NetworkTestnet,
		},
		"multi-part originator returns testnet": {
			args:            nil,
			originator:      "subdomain.example.com",
			expectedNetwork: sdk.NetworkTestnet,
		},
		"max single label length (63) across multiple parts": {
			originator:      strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63),
			args:            nil,
			expectedNetwork: sdk.NetworkTestnet,
		},
		"max total length (250 chars) should pass": {
			originator:      strings.Repeat("a", 250),
			args:            nil,
			expectedNetwork: sdk.NetworkTestnet,
		},
		"case-insensitive originator (if normalization is supported)": {
			originator:      strings.ToUpper(fixtures.DefaultOriginator),
			args:            nil,
			expectedNetwork: sdk.NetworkTestnet,
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
