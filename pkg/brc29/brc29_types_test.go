package brc29_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/brc29"
)

func TestKeyIDValidateAllowsWhitespaceComponents(t *testing.T) {
	whitespaceKeyID := brc29.KeyID{DerivationPrefix: "prefix with space", DerivationSuffix: "suffix\twith\ttabs"}

	err := whitespaceKeyID.Validate()

	require.NoError(t, err)
}

func TestKeyIDStringUsesSingleSpaceForValidatedComponents(t *testing.T) {
	stringKeyID := brc29.KeyID{DerivationPrefix: "prefix", DerivationSuffix: "suffix"}

	require.NoError(t, stringKeyID.Validate())
	require.Equal(t, "prefix suffix", stringKeyID.String())
}
