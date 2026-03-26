package wallet_test

import (
	"context"
	"strings"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func TestIsAuthenticatedOriginatorValidation(t *testing.T) {
	RunOriginatorValidationErrorTests(t,
		func(w *wallet.Wallet, ctx context.Context, originator string) (*sdk.AuthenticatedResult, error) {
			return w.IsAuthenticated(ctx, nil, originator)
		},
	)
}

func (s *WalletTestSuite) TestWalletIsAuthenticated() {
	successTestCases := map[string]struct {
		args       any
		originator string
	}{
		"nil args with default originator": {
			args:       nil,
			originator: fixtures.DefaultOriginator,
		},
		"empty struct args with default originator": {
			args:       struct{}{},
			originator: fixtures.DefaultOriginator,
		},
		"map args with default originator": {
			args:       map[string]string{"key": "value"},
			originator: fixtures.DefaultOriginator,
		},
		"nil args with simple originator": {
			args:       nil,
			originator: "testoriginator",
		},
		"nil args with multi-part originator": {
			args:       nil,
			originator: "subdomain.example.com",
		},
		"max single label length (63) across multiple parts": {
			originator: strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63),
			args:       map[string]any{"k": "v"},
		},
		"max total length (250 chars) should pass": {
			originator: strings.Repeat("a", 250),
			args:       struct{ Foo string }{Foo: "bar"},
		},
		"default originator still passes": {
			originator: fixtures.DefaultOriginator,
			args:       nil,
		},
		"case-insensitive originator (if normalization is supported)": {
			originator: strings.ToUpper(fixtures.DefaultOriginator),
			args:       nil,
		},
		"originator with 2 labels and simple map args": {
			originator: "example.com",
			args:       map[string]string{"key": "value"},
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
			result, err := aliceWallet.IsAuthenticated(t.Context(), test.args, test.originator)

			// then:
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Authenticated, "Should return authenticated as true")
		})
	}
}
