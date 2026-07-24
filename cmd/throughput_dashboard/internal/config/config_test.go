package config_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/cmd/throughput_dashboard/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/stretchr/testify/require"
)

func TestConfigFromEnvRequiresPrivateKey(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "")
	_, err := config.ConfigFromEnv()
	require.Error(t, err)
	require.Contains(t, err.Error(), "PRIVATE_KEY")
}

func TestConfigFromEnvMainnetDefaults(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "aa")
	t.Setenv("SERVER_URL", "")
	t.Setenv("BSV_NETWORK", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("TPS", "")
	t.Setenv("WORKERS", "")
	t.Setenv("SAMPLE_SECONDS", "")
	t.Setenv("ORIGINATOR", "")

	cfg, err := config.ConfigFromEnv()
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:8101", cfg.ServerURL)
	require.Equal(t, string(defs.NetworkMainnet), cfg.Network)
	require.Equal(t, "127.0.0.1:8200", cfg.HTTPAddr)
	require.Equal(t, 10, cfg.TPS)
	require.Equal(t, 0, cfg.Workers) // 0 = auto from TPS
	require.Equal(t, 1, cfg.SampleSeconds)
	require.Equal(t, "throughput-dashboard.local", cfg.Originator)
}

func TestConfigFromEnvOverrides(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "deadbeef")
	t.Setenv("SERVER_URL", "http://infra:8100")
	t.Setenv("BSV_NETWORK", "main")
	t.Setenv("HTTP_ADDR", "0.0.0.0:8200")
	t.Setenv("TPS", "50")
	t.Setenv("WORKERS", "16")
	t.Setenv("SAMPLE_SECONDS", "2")
	t.Setenv("ORIGINATOR", "custom.local")

	cfg, err := config.ConfigFromEnv()
	require.NoError(t, err)
	require.Equal(t, "http://infra:8100", cfg.ServerURL)
	require.Equal(t, "main", cfg.Network)
	require.Equal(t, "deadbeef", cfg.PrivateKeyHex)
	require.Equal(t, "0.0.0.0:8200", cfg.HTTPAddr)
	require.Equal(t, 50, cfg.TPS)
	require.Equal(t, 16, cfg.Workers)
	require.Equal(t, 2, cfg.SampleSeconds)
	require.Equal(t, "custom.local", cfg.Originator)
}

func TestConfigFromEnvRejectsNonPositiveTPS(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "aa")
	t.Setenv("TPS", "0")
	_, err := config.ConfigFromEnv()
	require.Error(t, err)
	require.Contains(t, err.Error(), "TPS")
}

func TestConfigFromEnvRejectsNonPositiveWorkers(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "aa")
	t.Setenv("WORKERS", "-1")
	_, err := config.ConfigFromEnv()
	require.Error(t, err)
	require.Contains(t, err.Error(), "WORKERS")
}

func TestConfigFromEnvRejectsNonPositiveSampleSeconds(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "aa")
	t.Setenv("SAMPLE_SECONDS", "0")
	_, err := config.ConfigFromEnv()
	require.Error(t, err)
	require.Contains(t, err.Error(), "SAMPLE_SECONDS")
}

func TestConfigFromEnvRejectsInvalidInt(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "aa")
	t.Setenv("TPS", "not-a-number")
	_, err := config.ConfigFromEnv()
	require.Error(t, err)
}

func TestConfigFromEnvRejectsInvalidNetwork(t *testing.T) {
	t.Setenv("PRIVATE_KEY", "aa")
	t.Setenv("BSV_NETWORK", "not-a-network")
	_, err := config.ConfigFromEnv()
	require.Error(t, err)
	require.Contains(t, err.Error(), "BSV_NETWORK")
}

func TestDemoThroughputMatchesLiveTestShape(t *testing.T) {
	tp := config.DemoThroughput()
	require.Equal(t, uint64(200), tp.ExpectedTxSizeBytes)
	require.Equal(t, uint64(0), tp.ExpectedOutputSatoshis)
	require.Equal(t, uint64(10), tp.TargetTPS)
	// Static demo profile still pins TargetPoolSize=500; live dashboard uses
	// DemoTargetPoolForTPS so the keeper tracks the UI TPS instead.
	require.Equal(t, uint64(500), tp.TargetPool())

	// Mainnet demo: 100 sat/kb → ceil(200/1000 * 100) = 20 sats.
	fee := config.DemoFeeModel(defs.NetworkMainnet)
	require.Equal(t, defs.SatPerKB, fee.Type)
	require.Equal(t, int64(100), fee.Value)

	denom, err := config.DemoDenomination(defs.NetworkMainnet)
	require.NoError(t, err)
	require.Equal(t, uint64(20), denom)
}

func TestDemoTargetPoolForTPS(t *testing.T) {
	// target_tps × expected_confirmation_seconds × pool_headroom_factor
	// DemoThroughput inherits 300s × 1.5 headroom from DefaultUTXOManagement.
	require.Equal(t, uint64(4_500), config.DemoTargetPoolForTPS(10))
	require.Equal(t, uint64(45_000), config.DemoTargetPoolForTPS(100))
	require.Equal(t, uint64(450), config.DemoTargetPoolForTPS(1))
	// Non-positive falls back to 10 TPS.
	require.Equal(t, uint64(4_500), config.DemoTargetPoolForTPS(0))
	require.Equal(t, uint64(4_500), config.DemoTargetPoolForTPS(-5))
}

func TestDemoFeeModelTSTNMatchesArcadeFloor(t *testing.T) {
	// Arcade DefaultMinFeePerKB is 100 sat/kb; wallet must match or posts get ARC 465.
	fee := config.DemoFeeModel(defs.NetworkTSTN)
	require.Equal(t, defs.SatPerKB, fee.Type)
	require.Equal(t, int64(100), fee.Value)

	// 200 B OP_RETURN shape → ceil(200/1000*100)=20 > MarginalFuelInputFee(15).
	denom, err := config.DemoDenomination(defs.NetworkTSTN)
	require.NoError(t, err)
	require.Equal(t, uint64(20), denom)

	// Same as mainnet demo fee/denom (host floor, not "TSTN is 1 sat/kb").
	mainDenom, err := config.DemoDenomination(defs.NetworkMainnet)
	require.NoError(t, err)
	require.Equal(t, mainDenom, denom)

	tp := config.DemoThroughput()
	require.NoError(t, (&defs.UTXOManagement{
		Strategy:   defs.StrategyThroughput,
		Throughput: tp,
	}).Validate(fee, defs.DefaultCommission()))
}
