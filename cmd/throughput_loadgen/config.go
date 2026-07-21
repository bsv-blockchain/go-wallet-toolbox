package main

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds loadgen runtime settings loaded from the environment.
type Config struct {
	ServerURL       string
	Network         string
	PrivateKeyHex   string
	Originator      string
	FaucetTxID      string
	TPS             int
	Workers         int
	WarmupSeconds   int
	DurationSeconds int
}

// ConfigFromEnv reads loadgen configuration from environment variables.
// Defaults apply when a variable is unset or empty. PRIVATE_KEY is required.
// TPS and Workers must be positive.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		ServerURL:       envOrDefault("SERVER_URL", "http://infra:8100"),
		Network:         envOrDefault("BSV_NETWORK", "test"),
		PrivateKeyHex:   os.Getenv("PRIVATE_KEY"),
		Originator:      envOrDefault("ORIGINATOR", "throughput-loadgen.local"),
		FaucetTxID:      os.Getenv("FAUCET_TXID"),
		TPS:             1000,
		Workers:         64,
		WarmupSeconds:   5,
		DurationSeconds: 0,
	}

	if cfg.PrivateKeyHex == "" {
		return Config{}, fmt.Errorf("PRIVATE_KEY is required")
	}

	var err error
	if cfg.TPS, err = envIntOrDefault("TPS", 1000); err != nil {
		return Config{}, err
	}
	if cfg.Workers, err = envIntOrDefault("WORKERS", 64); err != nil {
		return Config{}, err
	}
	if cfg.WarmupSeconds, err = envIntOrDefault("WARMUP_SECONDS", 5); err != nil {
		return Config{}, err
	}
	if cfg.DurationSeconds, err = envIntOrDefault("DURATION_SECONDS", 0); err != nil {
		return Config{}, err
	}

	if cfg.TPS <= 0 {
		return Config{}, fmt.Errorf("TPS must be > 0, got %d", cfg.TPS)
	}
	if cfg.Workers <= 0 {
		return Config{}, fmt.Errorf("WORKERS must be > 0, got %d", cfg.Workers)
	}

	return cfg, nil
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
