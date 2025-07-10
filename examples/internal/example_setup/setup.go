package example_setup

import (
	"context"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
)

type Setup struct {
	Environment Environment
	IdentityKey *ec.PublicKey
	PrivateKey  *ec.PrivateKey
}

type Environment struct {
	BSVNetwork defs.BSVNetwork `mapstructure:"bsv_network"`
	ServerURL  string          `mapstructure:"server_url"`
}

type SetupConfig struct {
	Network   defs.BSVNetwork `mapstructure:"network" yaml:"network"`
	ServerURL string          `mapstructure:"server_url" yaml:"server_url"`
	Alice     UserConfig      `mapstructure:"alice" yaml:"alice"`
	Bob       UserConfig      `mapstructure:"bob" yaml:"bob"`
}

type UserConfig struct {
	IdentityKey string `mapstructure:"identity_key" yaml:"identity_key"`
	PrivateKey  string `mapstructure:"private_key" yaml:"private_key"`
}

func defaultSetupConfig() SetupConfig {
	return SetupConfig{
		Network:   defs.NetworkTestnet,
		ServerURL: "",
		Alice:     UserConfig{},
		Bob:       UserConfig{},
	}
}

func (c *SetupConfig) Validate() error {
	if _, err := defs.ParseBSVNetworkStr(string(c.Network)); err != nil {
		return fmt.Errorf("invalid BSV network: %w", err)
	}

	if c.Network == "" {
		return fmt.Errorf("network is required")
	}

	if c.ServerURL == "" {
		return fmt.Errorf("server_url is required")
	}

	if c.Alice.IdentityKey == "" || c.Alice.PrivateKey == "" {
		return fmt.Errorf("alice.identity_key and alice.private_key are required")
	}

	if c.Bob.IdentityKey == "" || c.Bob.PrivateKey == "" {
		return fmt.Errorf("bob.identity_key and bob.private_key are required")
	}

	return nil
}

func (c *SetupConfig) ToYAMLFile(filename string) error {
	return config.ToYAMLFile(c, filename)
}

func loadConfig() (*SetupConfig, error) {
	configFile := "examples/internal/example_setup/examples-config.yaml"
	loader := config.NewLoader(defaultSetupConfig, "EXAMPLE_SETUP")

	err := loader.SetConfigFilePath(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to set config file path: %w", err)
	}

	cfg, err := loader.Load()
	if err != nil {
		//TODO: add error message description to run the config generator if the config file is not found.
		// Config generator will be part of another PR.
		return nil, fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}

	err = cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func CreateAlice() *Setup {
	cfg, err := loadConfig()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	err = cfg.Validate()
	if err != nil {
		panic(fmt.Errorf("config validation failed: %w", err))
	}

	privateKey, err := ec.PrivateKeyFromHex(cfg.Alice.PrivateKey)
	if err != nil {
		panic(fmt.Errorf("failed to parse Alice's private key: %w", err))
	}

	identityKey := privateKey.PubKey()

	if identityKey.ToDERHex() != cfg.Alice.IdentityKey {
		panic(fmt.Errorf("identity key does not match the public key derived from private key"))
	}

	return &Setup{
		Environment: Environment{
			BSVNetwork: cfg.Network,
			ServerURL:  cfg.ServerURL,
		},
		IdentityKey: identityKey,
		PrivateKey:  privateKey,
	}
}

func (s *Setup) CreateWallet(ctx context.Context) (*wallet.Wallet, func(), error) {
	storageClient, cleanup, err := storage.NewClient(s.Environment.ServerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	userWallet, err := wallet.New(s.Environment.BSVNetwork, s.PrivateKey, storageClient)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	show.Info("CreateWallet", s.IdentityKey.ToDERHex())
	return userWallet, cleanup, nil
}
