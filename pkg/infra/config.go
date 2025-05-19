package infra

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/config"
)

//go:generate go run ../../cmd/infra_config_gen/main.go -o ../../infra-config.example.yaml

// Config is the configuration for the "remote storage server" service (aka "infra")
type Config struct {
	// Name is the human-readable name of this storage server
	Name             string              `mapstructure:"name"`
	ServerPrivateKey string              `mapstructure:"server_private_key"`
	BSVNetwork       defs.BSVNetwork     `mapstructure:"bsv_network"`
	FeeModel         defs.FeeModel       `mapstructure:"fee_model"`
	DBConfig         defs.Database       `mapstructure:"db"`
	HTTPConfig       HTTPConfig          `mapstructure:"http"`
	Logging          LogConfig           `mapstructure:"logging"`
	Commission       defs.Commission     `mapstructure:"commission"`
	Services         defs.WalletServices `mapstructure:"wallet_services"`
	Monitor          defs.Monitor        `mapstructure:"monitor"`
}

// DBConfig is the configuration for the database
type DBConfig struct {
	Engine defs.DBType `mapstructure:"engine"`
}

// HTTPConfig is the configuration for the HTTP server related settings
type HTTPConfig struct {
	Port uint `mapstructure:"port"`
}

// LogConfig is the configuration for the logging
type LogConfig struct {
	Enabled bool            `mapstructure:"enabled"`
	Level   defs.LogLevel   `mapstructure:"level"`
	Handler defs.LogHandler `mapstructure:"handler"`
}

// Defaults returns the default configuration
func Defaults() Config {
	network := defs.NetworkMainnet

	return Config{
		Name:             "go-storage-server",
		ServerPrivateKey: "", // it is not optional, user must provide it

		BSVNetwork: network,
		DBConfig:   defs.DefaultDBConfig(),
		HTTPConfig: HTTPConfig{
			Port: 8100,
		},
		FeeModel: defs.DefaultFeeModel(),
		Logging: LogConfig{
			Enabled: true,
			Level:   defs.LogLevelInfo,
			Handler: defs.JSONHandler,
		},
		Commission: defs.DefaultCommission(),
		Services:   defs.DefaultServicesConfig(network),
		Monitor:    defs.DefaultMonitorConfig(),
	}
}

// OnPostLoad is called after the configuration is loaded
func (c *Config) OnPostLoad() error {
	c.Services.Chain = c.BSVNetwork

	// if testnet selected - switch to testnet-ARC if default config points to mainnet-ARC
	if c.BSVNetwork == defs.NetworkTestnet && c.Services.ArcConfig.URL == defs.ArcURL {
		c.Services.ArcConfig.URL = defs.ArcTestURL
		c.Services.ArcConfig.Token = defs.ArcTestToken
	}
	return nil
}

// Validate validates the whole configuration
func (c *Config) Validate() (err error) {
	if c.ServerPrivateKey == "" {
		return fmt.Errorf("server private key is required")
	}
	if c.BSVNetwork, err = defs.ParseBSVNetworkStr(string(c.BSVNetwork)); err != nil {
		return fmt.Errorf("invalid BSV network: %w", err)
	}

	if err = c.FeeModel.Validate(); err != nil {
		return fmt.Errorf("invalid fee model: %w", err)
	}

	if err = c.DBConfig.Validate(); err != nil {
		return fmt.Errorf("invalid DB config: %w", err)
	}

	if err = c.Logging.Validate(); err != nil {
		return fmt.Errorf("invalid HTTP config: %w", err)
	}

	if err = c.Commission.Validate(); err != nil {
		return fmt.Errorf("invalid commission config: %w", err)
	}

	if err = c.Services.Validate(); err != nil {
		return fmt.Errorf("invalid services config: %w", err)
	}

	if err = c.Monitor.Validate(); err != nil {
		return fmt.Errorf("invalid monitor config: %w", err)
	}

	return nil
}

// Validate validates the DB configuration
func (c *DBConfig) Validate() (err error) {
	if c.Engine, err = defs.ParseDBTypeStr(string(c.Engine)); err != nil {
		return fmt.Errorf("invalid DB engine: %w", err)
	}

	return nil
}

// Validate validates the HTTP configuration
func (c *LogConfig) Validate() (err error) {
	if c.Level, err = defs.ParseLogLevelStr(string(c.Level)); err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	if c.Handler, err = defs.ParseHandlerTypeStr(string(c.Handler)); err != nil {
		return fmt.Errorf("invalid log handler: %w", err)
	}

	return nil
}

// ToYAMLFile writes the configuration to a YAML file
func (c *Config) ToYAMLFile(filename string) error {
	err := config.ToYAMLFile(c, filename)
	if err != nil {
		return fmt.Errorf("failed to write config to file: %w", err)
	}
	return nil
}
