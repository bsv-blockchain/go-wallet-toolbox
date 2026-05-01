package certifier

import (
	"fmt"
	"strconv"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	servercommon "github.com/bsv-blockchain/go-wallet-toolbox/pkg/server"
)

// ServerSettings is the HTTP and network configuration for the certifier.
type ServerSettings struct {
	Port                string          `mapstructure:"port"`
	Network             defs.BSVNetwork `mapstructure:"network"`
	MaxRequestBodyBytes int64           `mapstructure:"max_request_body_bytes"`
}

// Config is the configuration loaded from YAML file for the certifier server.
type Config struct {
	Server          ServerSettings `mapstructure:"server"`
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
		Server: ServerSettings{
			Port:                "8080",
			Network:             defs.NetworkTestnet,
			MaxRequestBodyBytes: servercommon.DefaultMaxRequestBodyBytes,
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
	if err := validatePort(c.Server.Port); err != nil {
		return fmt.Errorf("invalid server.port: %w", err)
	}
	if c.Server.MaxRequestBodyBytes <= 0 {
		return fmt.Errorf("server.max_request_body_bytes must be greater than 0")
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

func validatePort(port string) error {
	const (
		minPort = 1
		maxPort = 65535
	)

	parsed, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("must be an integer: %w", err)
	}
	if parsed < minPort || parsed > maxPort {
		return fmt.Errorf("must be between %d and %d", minPort, maxPort)
	}
	return nil
}
