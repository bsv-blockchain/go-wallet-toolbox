package certifier

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// Config is the configuration loaded from YAML file for the certifier server.
type Config struct {
	Server struct {
		Port    string          `mapstructure:"port"`
		Network defs.BSVNetwork `mapstructure:"network"`
	} `mapstructure:"server"`
	CertifierWallet struct {
		PrivateKey string `mapstructure:"private_key"`
	} `mapstructure:"certifier_wallet"`
	Storage struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"storage"`
	Logging defs.LogConfig `mapstructure:"logging"`
}

// ConfigDefaults returns the default configuration.
func ConfigDefaults() Config {
	return Config{
		Server: struct {
			Port    string          `mapstructure:"port"`
			Network defs.BSVNetwork `mapstructure:"network"`
		}{
			Port:    "8080",
			Network: defs.NetworkTestnet,
		},
		Storage: struct {
			URL string `mapstructure:"url"`
		}{
			URL: "http://localhost:8100",
		},
		Logging: defs.DefaultLogConfig(),
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Storage.URL == "" {
		return fmt.Errorf("storage.url is required")
	}
	if c.CertifierWallet.PrivateKey == "" {
		return fmt.Errorf("certifier_wallet.private_key is required")
	}
	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("invalid logging config: %w", err)
	}
	return nil
}

// OnPostLoad is called after the configuration is loaded.
func (c *Config) OnPostLoad() error {
	var err error
	if c.Server.Network != "" {
		if c.Server.Network, err = defs.ParseBSVNetworkStr(string(c.Server.Network)); err != nil {
			return fmt.Errorf("invalid network: %w", err)
		}
	}
	return nil
}
