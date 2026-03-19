package example_setup

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"

	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// getConfigFilePath returns the absolute path to the config file
func getConfigFilePath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}

	examplesDir := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	return filepath.Join(examplesDir, "examples-config.yaml")
}

// defaultSetupConfig returns a default setup configuration
func defaultSetupConfig() SetupConfig {
	return SetupConfig{
		Network:          defs.NetworkTestnet,
		ServerURL:        "",
		ServerPrivateKey: "",
		Alice:            UserConfig{},
		Bob:              UserConfig{},
	}
}

// generateUserConfig creates a new user configuration with random keys
func generateUserConfig() (UserConfig, error) {
	privKey, err := ec.NewPrivateKey()
	if err != nil {
		return UserConfig{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	return UserConfig{
		IdentityKey: privKey.PubKey().ToDERHex(),
		PrivateKey:  hex.EncodeToString(privKey.Serialize()),
	}, nil
}

// generateConfig creates a new setup configuration with default values and random keys
func generateConfig() (*SetupConfig, error) {
	alice, err := generateUserConfig()
	if err != nil {
		return nil, fmt.Errorf("error generating Alice config: %w", err)
	}

	bob, err := generateUserConfig()
	if err != nil {
		return nil, fmt.Errorf("error generating Bob config: %w", err)
	}

	serverPrivKey, err := ec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("error generating server private key: %w", err)
	}

	cfg := &SetupConfig{
		Network:          defs.NetworkTestnet,
		ServerURL:        "", // Empty by default - will use local storage
		ServerPrivateKey: hex.EncodeToString(serverPrivKey.Serialize()),
		Alice:            alice,
		Bob:              bob,
	}

	err = config.ToYAMLFile(cfg, getConfigFilePath())
	if err != nil {
		return nil, fmt.Errorf("failed to save generated config to %s: %w", getConfigFilePath(), err)
	}

	return cfg, nil
}

// LoadConfig loads the configuration from the examples-config.yaml file
// if the file does not exist, it generates a new configuration and saves it to the file
func LoadConfig() (*SetupConfig, error) {
	configFile := getConfigFilePath()

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		show.Info("Config file not found, generating new configuration", configFile)

		cfg, err := generateConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to generate default config: %w", err)
		}

		show.Info("Generated new configuration file", configFile)
		return cfg, nil
	}

	loader := config.NewLoader(defaultSetupConfig, "EXAMPLE_SETUP")

	err := loader.SetConfigFilePath(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to set config file path: %w", err)
	}

	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}

	err = cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}
