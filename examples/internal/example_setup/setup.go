package example_setup

import (
	"context"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/spf13/viper"
)

type Setup struct {
	Environment Environment
	IdentityKey ec.PublicKey
	PrivateKey  ec.PrivateKey
}

type Environment struct {
	BSVNetwork defs.BSVNetwork `mapstructure:"bsv_network"`
	ServerURL  string          `mapstructure:"server_url"`
}

type SetupConfig struct {
	Network   defs.BSVNetwork `mapstructure:"network"`
	ServerURL string          `mapstructure:"server_url"`
	Alice     UserConfig      `mapstructure:"alice"`
	Bob       UserConfig      `mapstructure:"bob"`
}

type UserConfig struct {
	IdentityKey string `mapstructure:"identity_key"`
	PrivateKey  string `mapstructure:"private_key"`
}

func (c *SetupConfig) Validate() error {
	if _, err := defs.ParseBSVNetworkStr(string(c.Network)); err != nil {
		return fmt.Errorf("invalid BSV network: %w", err)
	}
	return nil
}

func loadConfig(configFile string) (*SetupConfig, error) {
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg SetupConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func CreateAlice() *Setup {
	cfg, err := loadConfig("examples/internal/example_setup/example-config.yaml")
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
		IdentityKey: *identityKey,
		PrivateKey:  *privateKey,
	}
}

func (s *Setup) CreateWallet(ctx context.Context) (*wallet.Wallet, func(), error) {
	storageClient, cleanup, err := storage.NewClient(s.Environment.ServerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	userWallet, err := wallet.New(s.Environment.BSVNetwork, &s.PrivateKey, storageClient)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	identityKey := s.IdentityKey.ToDERHex()
	user, err := storageClient.FindOrInsertUser(ctx, identityKey)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to find or insert user: %w", err)
	}

	fmt.Printf("CreateWallet: User %d: %s\n", user.User.UserID, identityKey)
	return userWallet, cleanup, nil
}
