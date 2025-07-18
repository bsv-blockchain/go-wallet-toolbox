package example_setup

import (
	"encoding/hex"
	"fmt"
	"os"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

func defaultSetupConfig() SetupConfig {
	return SetupConfig{
		Network:   defs.NetworkTestnet,
		ServerURL: "",
		Alice:     UserConfig{},
		Bob:       UserConfig{},
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

// generateDefaultConfig creates a new setup configuration with default values and random keys
func generateDefaultConfig() (*SetupConfig, error) {
	alice, err := generateUserConfig()
	if err != nil {
		return nil, fmt.Errorf("error generating Alice config: %w", err)
	}

	bob, err := generateUserConfig()
	if err != nil {
		return nil, fmt.Errorf("error generating Bob config: %w", err)
	}

	cfg := &SetupConfig{
		Network:   defs.NetworkTestnet,
		ServerURL: "http://localhost:8100",
		Alice:     alice,
		Bob:       bob,
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("generated config validation failed: %w", err)
	}

	return cfg, nil
}

// loadConfig loads the configuration from the examples-config.yaml file
// if the file does not exist, it generates a new configuration and saves it to the file
func loadConfig() (*SetupConfig, error) {
	const configFile = "examples/examples-config.yaml"

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		show.Info("Config file not found, generating new configuration", configFile)

		cfg, err := generateDefaultConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to generate default config: %w", err)
		}

		if err := cfg.ToYAMLFile(configFile); err != nil {
			return nil, fmt.Errorf("failed to save generated config to %s: %w", configFile, err)
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

	return &cfg, nil
}
