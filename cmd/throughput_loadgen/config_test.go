package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigFromEnvDefaults(t *testing.T) {
	// Clear all loadgen env vars so defaults apply.
	for _, key := range []string{
		"SERVER_URL", "BSV_NETWORK", "PRIVATE_KEY", "TPS", "WORKERS",
		"ORIGINATOR", "FAUCET_TXID", "WARMUP_SECONDS", "DURATION_SECONDS",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("PRIVATE_KEY", "aabbccdd")

	cfg, err := ConfigFromEnv()
	require.NoError(t, err)
	require.Equal(t, "http://infra:8100", cfg.ServerURL)
	require.Equal(t, "test", cfg.Network)
	require.Equal(t, "aabbccdd", cfg.PrivateKeyHex)
	require.Equal(t, 1000, cfg.TPS)
	require.Equal(t, 64, cfg.Workers)
	require.Equal(t, "throughput-loadgen.local", cfg.Originator)
	require.Equal(t, "", cfg.FaucetTxID)
	require.Equal(t, 5, cfg.WarmupSeconds)
	require.Equal(t, 0, cfg.DurationSeconds)
}

func TestConfigFromEnvRequiresPrivateKey(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "")
	_, err := ConfigFromEnv()
	require.Error(t, err)
	require.Contains(t, err.Error(), "PRIVATE_KEY")
}

func TestConfigFromEnvOverrides(t *testing.T) {
	t.Setenv("SERVER_URL", "http://localhost:8100")
	t.Setenv("BSV_NETWORK", "main")
	t.Setenv("PRIVATE_KEY", "deadbeef")
	t.Setenv("TPS", "250")
	t.Setenv("WORKERS", "8")
	t.Setenv("ORIGINATOR", "custom.local")
	t.Setenv("FAUCET_TXID", "abc123")
	t.Setenv("WARMUP_SECONDS", "10")
	t.Setenv("DURATION_SECONDS", "30")

	cfg, err := ConfigFromEnv()
	require.NoError(t, err)
	require.Equal(t, "http://localhost:8100", cfg.ServerURL)
	require.Equal(t, "main", cfg.Network)
	require.Equal(t, "deadbeef", cfg.PrivateKeyHex)
	require.Equal(t, 250, cfg.TPS)
	require.Equal(t, 8, cfg.Workers)
	require.Equal(t, "custom.local", cfg.Originator)
	require.Equal(t, "abc123", cfg.FaucetTxID)
	require.Equal(t, 10, cfg.WarmupSeconds)
	require.Equal(t, 30, cfg.DurationSeconds)
}

func TestConfigFromEnvRejectsNonPositiveTPS(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "aabb")
	t.Setenv("TPS", "0")
	_, err := ConfigFromEnv()
	require.Error(t, err)
	require.Contains(t, err.Error(), "TPS")
}

func TestConfigFromEnvRejectsNonPositiveWorkers(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "aabb")
	t.Setenv("WORKERS", "-1")
	_, err := ConfigFromEnv()
	require.Error(t, err)
	require.Contains(t, err.Error(), "WORKERS")
}

func TestConfigFromEnvRejectsInvalidInt(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "aabb")
	t.Setenv("TPS", "not-a-number")
	_, err := ConfigFromEnv()
	require.Error(t, err)
}
