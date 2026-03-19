package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/bsv-blockchain/certifier-server-example/internal/constants"
	"github.com/go-softwarelab/common/pkg/to"
	"gopkg.in/yaml.v3"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

type Config struct {
	Server struct {
		Port    string `yaml:"port"`
		Network string `yaml:"network"`
	} `yaml:"server"`

	CertifierWallet struct {
		IdentityKey string `yaml:"identity_key"`
		PrivateKey  string `yaml:"private_key"`
	} `yaml:"certifier_wallet"`

	UserWallet struct {
		IdentityKey string `yaml:"identity_key"`
		PrivateKey  string `yaml:"private_key"`
	} `yaml:"user_wallet"`

	Storage struct {
		URL        string `yaml:"url"`
		PrivateKey string `yaml:"private_key"`
	} `yaml:"storage"`
}

func LoadConfig(path string, log *slog.Logger) (*Config, error) {
	path = to.IfThen(path != "", path).ElseThen(getConfigFilePath())

	data, err := os.ReadFile(path) //nolint:gosec // path is resolved from environment/defaults, not user input
	if err != nil {
		log.Warn("Could not read config file, using defaults", "error", err)
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("could not parse YAML: %w", err)
	}

	return &cfg, nil
}

func getConfigFilePath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}

	examplesDir := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	return filepath.Join(examplesDir, constants.ConfigFileName)
}

func (c *Config) Validate() error {
	if c.Server.Port == "" {
		c.Server.Port = constants.DefaultServerPort
	}

	if c.Server.Network == "" {
		c.Server.Network = to.String(defs.NetworkTestnet)
	}

	if c.Storage.PrivateKey == "" {
		return fmt.Errorf("server private key is required")
	}

	if c.CertifierWallet.PrivateKey == "" {
		return fmt.Errorf("certifier wallet private key is required")
	}

	if c.CertifierWallet.IdentityKey == "" {
		return fmt.Errorf("certifier wallet identity key is required")
	}

	if c.UserWallet.PrivateKey == "" {
		return fmt.Errorf("user wallet private key is required")
	}

	if c.UserWallet.IdentityKey == "" {
		return fmt.Errorf("user wallet identity key is required")
	}

	return nil
}
