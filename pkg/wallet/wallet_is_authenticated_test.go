package wallet_test

import (
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletIsAuthenticatedArgsValidation(t *testing.T) {
	errorTestCases := map[string]struct {
		originator string
		args       any
	}{
		"too long originator": {
			originator: strings.Repeat("a", 251),
			args:       nil,
		},
		"too long originator part": {
			originator: "a." + strings.Repeat("b", 64) + ".c",
			args:       nil,
		},
		"empty part in originator": {
			originator: "a..c",
			args:       nil,
		},
	}

	for name, test := range errorTestCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given, then, cleanup := testabilities.New(t)
			defer cleanup()
			aliceWallet := given.AliceWalletWithStorage(testabilities.StorageTypeMocked)

			// when:
			result, err := aliceWallet.IsAuthenticated(t.Context(), test.args, test.originator)

			// then:
			then.Result(result).HasError(err)

			then.Storage().HadNoInteraction()
		})
	}
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
			assert.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Authenticated, "Should return authenticated as true")
		})
	}
}
