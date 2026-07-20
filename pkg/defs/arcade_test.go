package defs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func TestDefaultServicesConfigArcadeMainnet(t *testing.T) {
	// when:
	cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)

	// then:
	require.True(t, cfg.Arcade.Enabled)
	require.Equal(t, defs.ArcadeURL, cfg.Arcade.URL)
	require.Equal(t, cfg.Arcade.URL, cfg.Arcade.EventsURL)
	require.True(t, cfg.Arcade.FullStatusUpdates)
	require.Equal(t, uint(3), cfg.Arcade.CircuitBreaker.FailureThreshold)
	require.Equal(t, uint(30), cfg.Arcade.CircuitBreaker.HealthProbeIntervalSeconds)

	require.True(t, cfg.ArcGorillaPoolConfig.Enabled)
	require.Equal(t, defs.GorillaPoolArcURL, cfg.ArcGorillaPoolConfig.URL)
}

func TestDefaultServicesConfigArcadeTestnet(t *testing.T) {
	// when:
	cfg := defs.DefaultServicesConfig(defs.NetworkTestnet)

	// then:
	require.False(t, cfg.Arcade.Enabled)
	require.False(t, cfg.ArcGorillaPoolConfig.Enabled)

	// and: URLs are empty off mainnet, so enabling without an explicit URL
	// cannot silently hit mainnet (Validate forces an explicit URL)
	require.Empty(t, cfg.Arcade.URL)
	require.Empty(t, cfg.Arcade.EventsURL)
	require.Empty(t, cfg.ArcGorillaPoolConfig.URL)
}

func TestArcadeValidate(t *testing.T) {
	t.Run("disabled config passes regardless of URL", func(t *testing.T) {
		// given:
		arcade := defs.Arcade{Enabled: false}

		// when:
		err := arcade.Validate()

		// then:
		require.NoError(t, err)
	})

	t.Run("enabled config without URL returns error", func(t *testing.T) {
		// given:
		arcade := defs.Arcade{Enabled: true}

		// when:
		err := arcade.Validate()

		// then:
		require.Error(t, err)
		require.Contains(t, err.Error(), "url is empty")
	})

	t.Run("enabled config with localhost callback URL returns error", func(t *testing.T) {
		// given:
		arcade := defs.Arcade{
			Enabled:     true,
			URL:         defs.ArcadeURL,
			CallbackURL: "http://localhost:8080/callback",
		}

		// when:
		err := arcade.Validate()

		// then:
		require.Error(t, err)
		require.Contains(t, err.Error(), "localhost")
	})

	t.Run("enabled config with external callback URL passes", func(t *testing.T) {
		// given:
		arcade := defs.Arcade{
			Enabled:     true,
			URL:         defs.ArcadeURL,
			CallbackURL: "https://example.com/callback",
		}

		// when:
		err := arcade.Validate()

		// then:
		require.NoError(t, err)
	})

	t.Run("enabled config with URL passes and defaults EventsURL", func(t *testing.T) {
		// given:
		arcade := defs.Arcade{
			Enabled: true,
			URL:     defs.ArcadeURL,
		}

		// when:
		err := arcade.Validate()

		// then:
		require.NoError(t, err)
		require.Equal(t, defs.ArcadeURL, arcade.EventsURL)
	})
}

func TestWalletServicesValidateWithArcade(t *testing.T) {
	t.Run("default mainnet config validates", func(t *testing.T) {
		// given:
		cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)

		// when:
		err := cfg.Validate()

		// then:
		require.NoError(t, err)
	})

	t.Run("invalid Arcade config fails WalletServices validation", func(t *testing.T) {
		// given:
		cfg := defs.DefaultServicesConfig(defs.NetworkMainnet)
		cfg.Arcade.Enabled = true
		cfg.Arcade.URL = ""

		// when:
		err := cfg.Validate()

		// then:
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid Arcade config")
	})
}
