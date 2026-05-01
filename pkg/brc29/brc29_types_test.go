package brc29_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
)

func TestKeyIDValidateRejectsWhitespaceComponents(t *testing.T) {
	tests := map[string]brc29.KeyID{
		"prefix contains space": {DerivationPrefix: "a b", DerivationSuffix: "c"},
		"suffix contains space": {DerivationPrefix: "a", DerivationSuffix: "b c"},
		"prefix contains tab":   {DerivationPrefix: "a\tb", DerivationSuffix: "c"},
	}
	for name, keyID := range tests {
		t.Run(name, func(t *testing.T) {
			err := keyID.Validate()

			require.ErrorContains(t, err, "must not contain whitespace")
		})
	}
}

func TestKeyIDStringUsesSingleSpaceForValidatedComponents(t *testing.T) {
	keyID := brc29.KeyID{DerivationPrefix: "prefix", DerivationSuffix: "suffix"}

	require.NoError(t, keyID.Validate())
	require.Equal(t, "prefix suffix", keyID.String())
}
