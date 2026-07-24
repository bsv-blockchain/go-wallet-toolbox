// Package config loads throughput-dashboard settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// Config holds dashboard runtime settings.
type Config struct {
	ServerURL     string
	Network       string
	PrivateKeyHex string
	Originator    string
	HTTPAddr      string
	TPS           int
	Workers       int
	SampleSeconds int
}

// ConfigFromEnv reads configuration from environment variables.
// PRIVATE_KEY is required. Defaults are mainnet-first and demo-safe.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		ServerURL:     envOrDefault("SERVER_URL", "http://127.0.0.1:8101"),
		Network:       envOrDefault("BSV_NETWORK", string(defs.NetworkMainnet)),
		PrivateKeyHex: os.Getenv("PRIVATE_KEY"),
		Originator:    envOrDefault("ORIGINATOR", "throughput-dashboard.local"),
		HTTPAddr:      envOrDefault("HTTP_ADDR", "127.0.0.1:8200"),
		TPS:           10,
		Workers:       0, // 0 → derive from TPS in stream.WorkersForTPS
		SampleSeconds: 1,
	}

	if cfg.PrivateKeyHex == "" {
		return Config{}, fmt.Errorf("PRIVATE_KEY is required")
	}

	var err error
	if cfg.TPS, err = envIntOrDefault("TPS", 10); err != nil {
		return Config{}, err
	}
	// WORKERS=0 or unset → auto from TPS. Positive value is an explicit override.
	if cfg.Workers, err = envIntOrDefault("WORKERS", 0); err != nil {
		return Config{}, err
	}
	if cfg.SampleSeconds, err = envIntOrDefault("SAMPLE_SECONDS", 1); err != nil {
		return Config{}, err
	}

	if cfg.TPS <= 0 {
		return Config{}, fmt.Errorf("TPS must be > 0, got %d", cfg.TPS)
	}
	// Workers may be 0 (auto-derived from TPS). Negative is invalid.
	if cfg.Workers < 0 {
		return Config{}, fmt.Errorf("WORKERS must be >= 0 (0 = auto from TPS), got %d", cfg.Workers)
	}
	if cfg.SampleSeconds <= 0 {
		return Config{}, fmt.Errorf("SAMPLE_SECONDS must be > 0, got %d", cfg.SampleSeconds)
	}
	if _, err = defs.ParseBSVNetworkStr(cfg.Network); err != nil {
		return Config{}, fmt.Errorf("BSV_NETWORK: %w", err)
	}

	return cfg, nil
}

// DemoThroughput returns the fuel profile for the local demo dashboard.
// Shape is 200 B OP_RETURN / 0 output sats; fee rate (and thus denomination)
// depends on the network — see DemoFeeModel.
//
// TargetPoolSize stays at a modest static default for callers that only need
// the shape; the live dashboard sizes inventory via DemoTargetPoolForTPS from
// the UI/env TPS so FuelKeeper tracks the stream rate.
func DemoThroughput() defs.Throughput {
	base := defs.DefaultUTXOManagement().Throughput
	base.ExpectedTxSizeBytes = 200
	base.ExpectedOutputSatoshis = 0
	base.TargetTPS = 10
	base.TargetPoolSize = 500 // static fallback; live path uses DemoTargetPoolForTPS
	base.FanoutMaxTxsPerRound = 50
	return base
}

// DemoFeeModel is the fee rate the demo dashboard assumes for fuel denomination.
// Mainnet/test/ttn demos use the toolbox default (100 sat/kb). TSTN network policy
// is 1 sat/kb; at that rate we pin denomination_satoshis to 2 (see
// infra-config-docker-throughput-tstn.yaml) so fuel exceeds the 1-sat input floor.
func DemoFeeModel(network defs.BSVNetwork) defs.FeeModel {
	if network == defs.NetworkTSTN {
		return defs.FeeModel{Type: defs.SatPerKB, Value: 1}
	}
	return defs.DefaultFeeModel()
}

// DemoDenomination resolves fuel UTXO size for the demo profile on the given network.
func DemoDenomination(network defs.BSVNetwork) (uint64, error) {
	tp := DemoThroughput()
	if network == defs.NetworkTSTN {
		// Must match infra-config-docker-throughput-tstn.yaml denomination_satoshis.
		tp.DenominationSatoshis = 2
	}
	return tp.Denomination(DemoFeeModel(network), defs.DefaultCommission())
}

// DemoTargetPoolForTPS sizes the FuelKeeper inventory target from a stream TPS
// setting: target_tps × expected_confirmation_seconds × pool_headroom_factor
// (same identity as defs.Throughput.TargetPool with TargetPoolSize left unset).
//
// Example with DemoThroughput defaults (300s confirmation, 1.5 headroom):
//
//	10 TPS  → 4_500 fuel UTXOs
//	100 TPS → 45_000 fuel UTXOs
func DemoTargetPoolForTPS(tps int) uint64 {
	if tps <= 0 {
		tps = 10
	}
	tp := DemoThroughput()
	tp.TargetTPS = uint64(tps)
	tp.TargetPoolSize = 0 // force derivation from TPS × confirmation × headroom
	return tp.TargetPool()
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid integer %q: %w", key, v, err)
	}
	return n, nil
}
