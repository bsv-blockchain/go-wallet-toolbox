package defs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestDefaultServicesConfigPerNetwork(t *testing.T) {
	t.Run("main enables Arcade+GorillaPool+WoC, ChainTracks off but arcade-derived", func(t *testing.T) {
		cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)

		require.Equal(t, defs.ArcURL, cfg.ArcConfig.URL)
		require.Equal(t, defs.ArcToken, cfg.ArcConfig.Token)
		require.True(t, cfg.Arcade.Enabled)
		require.Equal(t, defs.ArcadeURL, cfg.Arcade.URL)
		require.True(t, cfg.ArcGorillaPoolConfig.Enabled)
		require.True(t, cfg.WhatsOnChain.Enabled)
		require.False(t, cfg.ChaintracksClient.Enabled)
		require.Equal(t, "https://arcade-v2-us-1.bsvblockchain.tech/chaintracks", cfg.ChaintracksClient.RemoteURL)
	})

	t.Run("test keeps testnet ARC, Arcade off, WoC on", func(t *testing.T) {
		cfg := defs.DefaultServicesConfig(defs.NetworkTestnet)

		require.Equal(t, defs.ArcTestURL, cfg.ArcConfig.URL)
		require.Equal(t, defs.ArcTestToken, cfg.ArcConfig.Token)
		require.False(t, cfg.Arcade.Enabled)
		require.Empty(t, cfg.Arcade.URL)
		require.False(t, cfg.ArcGorillaPoolConfig.Enabled)
		require.True(t, cfg.WhatsOnChain.Enabled)
		require.False(t, cfg.ChaintracksClient.Enabled)
	})

	t.Run("ttn points ARC+Arcade+ChainTracks at the public ttn arcade host", func(t *testing.T) {
		cfg := defs.DefaultServicesConfig(defs.NetworkTTN)

		require.Equal(t, defs.ArcadeTTNURL, cfg.ArcConfig.URL)
		require.True(t, cfg.Arcade.Enabled)
		require.Equal(t, defs.ArcadeTTNURL, cfg.Arcade.URL)
		require.Equal(t, defs.ArcadeTTNURL, cfg.Arcade.EventsURL)
		require.False(t, cfg.ArcGorillaPoolConfig.Enabled)
		require.True(t, cfg.WhatsOnChain.Enabled)
		require.True(t, cfg.ChaintracksClient.Enabled)
		require.Equal(t, defs.ChaintracksClientModeRemote, cfg.ChaintracksClient.Mode)
		require.Equal(t, "https://arcade-v2-ttn-us-1.bsvblockchain.tech/chaintracks", cfg.ChaintracksClient.RemoteURL)

		require.NoError(t, cfg.Validate())
	})

	t.Run("tstn reads endpoints from env, disables WhatsOnChain", func(t *testing.T) {
		t.Setenv(defs.EnvTstnArcadeURL, "https://arcade.example.tstn")
		t.Setenv(defs.EnvTstnChaintracksURL, "")

		cfg := defs.DefaultServicesConfig(defs.NetworkTSTN)

		require.Equal(t, "https://arcade.example.tstn", cfg.ArcConfig.URL)
		require.True(t, cfg.Arcade.Enabled)
		require.Equal(t, "https://arcade.example.tstn", cfg.Arcade.URL)
		require.False(t, cfg.ArcGorillaPoolConfig.Enabled)
		require.False(t, cfg.WhatsOnChain.Enabled, "tstn has no WhatsOnChain service")
		require.True(t, cfg.ChaintracksClient.Enabled)
		// chaintracks falls back to the arcade host when TSTN_CHAINTRACKS_URL is unset.
		require.Equal(t, "https://arcade.example.tstn/chaintracks", cfg.ChaintracksClient.RemoteURL)

		require.NoError(t, cfg.Validate())
	})

	t.Run("tstn honors an explicit TSTN_CHAINTRACKS_URL", func(t *testing.T) {
		t.Setenv(defs.EnvTstnArcadeURL, "https://arcade.example.tstn")
		t.Setenv(defs.EnvTstnChaintracksURL, "https://ct.example.tstn/v1")

		cfg := defs.DefaultServicesConfig(defs.NetworkTSTN)

		require.Equal(t, "https://ct.example.tstn/v1", cfg.ChaintracksClient.RemoteURL)
		require.NoError(t, cfg.Validate())
	})

	t.Run("tstn without env vars fails validation with an actionable message", func(t *testing.T) {
		t.Setenv(defs.EnvTstnArcadeURL, "")
		t.Setenv(defs.EnvTstnChaintracksURL, "")

		cfg := defs.DefaultServicesConfig(defs.NetworkTSTN)

		require.False(t, cfg.WhatsOnChain.Enabled)
		require.Empty(t, cfg.Arcade.URL)

		err := cfg.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), defs.EnvTstnArcadeURL)
	})
}
