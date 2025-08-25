package config

import (
	"fmt"

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
