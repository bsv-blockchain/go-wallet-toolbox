package brc29_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
)

func TestKeyIDValidateAllowsWhitespaceComponents(t *testing.T) {
	keyID := brc29.KeyID{DerivationPrefix: "prefix with space", DerivationSuffix: "suffix\twith\ttabs"}

	err := keyID.Validate()

	require.NoError(t, err)
}

func TestKeyIDStringUsesSingleSpaceForValidatedComponents(t *testing.T) {
	keyID := brc29.KeyID{DerivationPrefix: "prefix", DerivationSuffix: "suffix"}

	require.NoError(t, keyID.Validate())
	require.Equal(t, "prefix suffix", keyID.String())
}
