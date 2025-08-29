package example_setup

import (
	"context"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox/examples/internal/show"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Setup struct {
	Environment      Environment
	IdentityKey      *ec.PublicKey
	PrivateKey       *ec.PrivateKey
	ServerPrivateKey string
	storageInfra     *StorageInfra
}

type Environment struct {
	BSVNetwork defs.BSVNetwork `mapstructure:"bsv_network"`
	ServerURL  string          `mapstructure:"server_url"`
}

type SetupConfig struct {
	Network          defs.BSVNetwork `mapstructure:"network"`
	ServerURL        string          `mapstructure:"server_url"`
	ServerPrivateKey string          `mapstructure:"server_private_key"`
	Alice            UserConfig      `mapstructure:"alice"`
	Bob              UserConfig      `mapstructure:"bob"`
}

type UserConfig struct {
	IdentityKey string `mapstructure:"identity_key"`
	PrivateKey  string `mapstructure:"private_key"`
}

func (u *UserConfig) Verify() error {
	if len(u.IdentityKey) == 0 {
		return fmt.Errorf("identity key value is required")
	}

	if len(u.PrivateKey) == 0 {
		return fmt.Errorf("private key value is required")
	}

	return nil
}

func (c *SetupConfig) Validate() error {
	if _, err := defs.ParseBSVNetworkStr(string(c.Network)); err != nil {
		return fmt.Errorf("invalid BSV network: %w", err)
	}

	if c.ServerPrivateKey == "" {
		return fmt.Errorf("server_private_key is required")
	}

	if err := c.Alice.Verify(); err != nil {
		return fmt.Errorf("alice user config is invalid: %w", err)
	}

	if err := c.Bob.Verify(); err != nil {
		return fmt.Errorf("bob user config is invalid: %w", err)
	}

	return nil
}

// CreateAlice creates a new Setup struct with the Alice's identity key and private key
// It loads the configuration from the examples-config.yaml file and validates the config
// It then creates a new wallet for Alice and returns the Setup struct
func CreateAlice() *Setup {
	cfg, err := LoadConfig()
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
		IdentityKey:      identityKey,
		PrivateKey:       privateKey,
		ServerPrivateKey: cfg.ServerPrivateKey,
	}
}

// CreateWallet creates a new wallet for the user
// It connects to the server and creates a new wallet
// It returns the wallet and a cleanup function, panicking if wallet creation fails
func (s *Setup) CreateWallet(ctx context.Context) (*wallet.Wallet, func()) {
	var storageClient wdk.WalletStorageProvider
	var cleanup func()

	if s.Environment.ServerURL != "" {
		// Use remote server when URL is provided
		client, clientCleanup, err := storage.NewClient(s.Environment.ServerURL)
		if err != nil {
			panic(fmt.Errorf("failed to connect to server: %w", err))
		}
		storageClient = client
		cleanup = clientCleanup
	} else {
		// Use local storage when no URL is provided
		storage, err := CreateLocalStorage(ctx, s.Environment.BSVNetwork, s.ServerPrivateKey)
		if err != nil {
			panic(fmt.Errorf("failed to create local storage: %w", err))
		}
		s.storageInfra = storage
		storageClient = storage.Provider
		cleanup = func() {
			if s.storageInfra != nil && s.storageInfra.Monitor != nil {
				s.storageInfra.Monitor.Stop()
			}
		}
	}

	userWallet, err := wallet.New(s.Environment.BSVNetwork, s.PrivateKey, storageClient)
	if err != nil {
		cleanup()
		panic(fmt.Errorf("failed to create wallet: %w", err))
	}

	show.Info("CreateWallet", s.IdentityKey.ToDERHex())
	return userWallet, cleanup
}
