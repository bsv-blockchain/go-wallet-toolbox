package defs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestTstnArcadeURL(t *testing.T) {
	t.Run("returns trimmed env value when set", func(t *testing.T) {
		t.Setenv(defs.EnvTstnArcadeURL, "  https://arcade.example.tstn  ")
		require.Equal(t, "https://arcade.example.tstn", defs.TstnArcadeURL())
	})

	t.Run("returns empty when unset", func(t *testing.T) {
		t.Setenv(defs.EnvTstnArcadeURL, "")
		require.Empty(t, defs.TstnArcadeURL())
	})
}

func TestTstnChaintracksURL(t *testing.T) {
	t.Run("uses explicit TSTN_CHAINTRACKS_URL when set", func(t *testing.T) {
		t.Setenv(defs.EnvTstnArcadeURL, "https://arcade.example.tstn")
		t.Setenv(defs.EnvTstnChaintracksURL, "https://ct.example.tstn/custom")

		got, err := defs.TstnChaintracksURL()
		require.NoError(t, err)
		require.Equal(t, "https://ct.example.tstn/custom", got)
	})

	t.Run("falls back to ${TSTN_ARCADE_URL}/chaintracks/v1 when chaintracks unset", func(t *testing.T) {
		t.Setenv(defs.EnvTstnArcadeURL, "https://arcade.example.tstn/")
		t.Setenv(defs.EnvTstnChaintracksURL, "")

		got, err := defs.TstnChaintracksURL()
		require.NoError(t, err)
		// trailing slash on the arcade URL is normalised away before appending the path.
		require.Equal(t, "https://arcade.example.tstn/chaintracks/v1", got)
	})

	t.Run("errors when neither variable is set", func(t *testing.T) {
		t.Setenv(defs.EnvTstnArcadeURL, "")
		t.Setenv(defs.EnvTstnChaintracksURL, "")

		_, err := defs.TstnChaintracksURL()
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.EnvTstnChaintracksURL)
		require.Contains(t, err.Error(), defs.EnvTstnArcadeURL)
	})
}
