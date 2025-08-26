package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type Config struct {
	Port int `mapstructure:"port"`

	// Faucet configuration
	ServerPrivateKey string          `mapstructure:"server_private_key"`
	FaucetPrivateKey string          `mapstructure:"faucet_private_key"`
	Network          defs.BSVNetwork `mapstructure:"network"`
	ServerURL        string          `mapstructure:"server_url"` // URL of wallet toolbox storage server
}

func Defaults() Config {
	return Config{
		Port:             8080,
		ServerPrivateKey: "",
		FaucetPrivateKey: "",
		Network:          defs.NetworkTestnet,
		ServerURL:        "http://127.0.0.1:8100",
	}
}

// Load loads configuration from environment variables with defaults
func Load() (Config, error) {
	cfg := Defaults()

	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}

	if serverKey := os.Getenv("SERVER_PRIVATE_KEY"); serverKey != "" {
		cfg.ServerPrivateKey = serverKey
	}

	if faucetKey := os.Getenv("FAUCET_PRIVATE_KEY"); faucetKey != "" {
		cfg.FaucetPrivateKey = faucetKey
	}

	if network := os.Getenv("NETWORK"); network != "" {
		cfg.Network = defs.BSVNetwork(network)
	}

	if serverURL := os.Getenv("SERVER_URL"); serverURL != "" {
		cfg.ServerURL = serverURL
	}

	return cfg, nil
}

// Validate normalizes and validates loaded configuration values.
func (c *Config) Validate() error {
	var err error

	if c.ServerPrivateKey == "" {
		return fmt.Errorf("server_private_key is required")
	}

	if c.FaucetPrivateKey == "" {
		return fmt.Errorf("faucet_private_key is required")
	}

	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if c.ServerURL == "" {
		return fmt.Errorf("server_url is required")
	}

	if c.Network, err = defs.ParseBSVNetworkStr(string(c.Network)); err != nil {
		return fmt.Errorf("invalid network: %w", err)
	}

	return nil
}
