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
		Workers:       8,
		SampleSeconds: 1,
	}

	if cfg.PrivateKeyHex == "" {
		return Config{}, fmt.Errorf("PRIVATE_KEY is required")
	}

	var err error
	if cfg.TPS, err = envIntOrDefault("TPS", 10); err != nil {
		return Config{}, err
	}
	if cfg.Workers, err = envIntOrDefault("WORKERS", 8); err != nil {
		return Config{}, err
	}
	if cfg.SampleSeconds, err = envIntOrDefault("SAMPLE_SECONDS", 1); err != nil {
		return Config{}, err
	}

	if cfg.TPS <= 0 {
		return Config{}, fmt.Errorf("TPS must be > 0, got %d", cfg.TPS)
	}
	if cfg.Workers <= 0 {
		return Config{}, fmt.Errorf("WORKERS must be > 0, got %d", cfg.Workers)
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
// Denomination shape matches the live-test OP_RETURN config (200 B / 0 output sats
// → 20 sat fuel). Target pool is intentionally small so FuelKeeper can refill
// from a modest deposit without minting hundreds of thousands of UTXOs.
func DemoThroughput() defs.Throughput {
	base := defs.DefaultUTXOManagement().Throughput
	base.ExpectedTxSizeBytes = 200
	base.ExpectedOutputSatoshis = 0
	base.TargetTPS = 10
	base.TargetPoolSize = 500 // explicit demo pool (not 1000 TPS × 300s × 1.5)
	base.FanoutMaxTxsPerRound = 50
	return base
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
