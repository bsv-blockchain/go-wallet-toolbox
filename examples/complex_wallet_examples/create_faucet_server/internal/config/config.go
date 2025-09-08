package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

const (
	EnvPort                 = "PORT"
	EnvFaucetPrivateKey     = "FAUCET_PRIVATE_KEY"
	EnvNetwork              = "NETWORK"
	EnvMaxFaucetTotalAmount = "MAX_FAUCET_TOTAL_AMOUNT"
)

type Config struct {
	Port                 int             `mapstructure:"port"`
	FaucetPrivateKey     string          `mapstructure:"faucet_private_key"`
	Network              defs.BSVNetwork `mapstructure:"network"`
	MaxFaucetTotalAmount uint64          `mapstructure:"max_faucet_total_amount"` // 0 means unlimited
}

func Defaults() Config {
	return Config{
		Port:                 8080,
		FaucetPrivateKey:     "",
		Network:              defs.NetworkTestnet,
		MaxFaucetTotalAmount: 0,
	}
}

// Load loads configuration from environment variables with defaults
func Load() (Config, error) {
	cfg := Defaults()

	if port := os.Getenv(EnvPort); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}

	if faucetKey := os.Getenv(EnvFaucetPrivateKey); faucetKey != "" {
		cfg.FaucetPrivateKey = faucetKey
	}

	if network := os.Getenv(EnvNetwork); network != "" {
		cfg.Network = defs.BSVNetwork(network)
	}

	if v := os.Getenv(EnvMaxFaucetTotalAmount); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			cfg.MaxFaucetTotalAmount = n
		}
	}

	return cfg, nil
}

// Validate normalizes and validates loaded configuration values.
func (c *Config) Validate() error {
	var err error

	if c.FaucetPrivateKey == "" {
		return fmt.Errorf("faucet_private_key is required (set %s)", EnvFaucetPrivateKey)
	}

	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535 (set %s)", EnvPort)
	}

	if c.Network, err = defs.ParseBSVNetworkStr(string(c.Network)); err != nil {
		return fmt.Errorf("invalid network (set %s): %w", EnvNetwork, err)
	}

	return nil
}
