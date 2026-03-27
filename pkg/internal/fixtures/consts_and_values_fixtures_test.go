package fixtures_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
)

func TestDerivationParts(t *testing.T) {
	tests := map[string]struct {
		derivation string
	}{
		"prefix": {
			derivation: fixtures.DerivationPrefix,
		},
		"suffix": {
			derivation: fixtures.DerivationSuffix,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bin, err := base64.StdEncoding.DecodeString(test.derivation)
			require.NoError(t, err)

			backToBase64Str := base64.StdEncoding.EncodeToString(bin)
			message := "Derivation string should be equal after decoding and encoding back to Base64. " +
				"If it is not, it means the input string is not the CANONICAL Base64 string. " +
				"Since such conversion (to binary and back to string) is done for wallet tests, it is important to ensure that the string is in the correct format."
			require.Equal(t, test.derivation, backToBase64Str, message)
		})
	}
}
