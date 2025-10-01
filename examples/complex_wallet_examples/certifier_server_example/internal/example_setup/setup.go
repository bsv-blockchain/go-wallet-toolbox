package example_setup

import (
	"context"
	"fmt"
	"log/slog"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

type Setup struct {
	Environment      Environment
	IdentityKey      *ec.PublicKey
	PrivateKey       *ec.PrivateKey
	ServerPrivateKey string
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

func (s *Setup) CreateWallet(ctx context.Context, privkey *ec.PrivateKey) (*wallet.Wallet, func()) {
	var storageProvider wdk.WalletStorageProvider
	var cleanup func()
	var err error

	remoteStorage := s.Environment.ServerURL != ""

	if remoteStorage {
		slog.Info("Using remote storage", s.Environment.ServerURL)
		storageProvider, cleanup, err = storage.NewClient(s.Environment.ServerURL)
	} else {
		slog.Info("Using local storage", SQLiteStorageFile)
		storageProvider, cleanup, err = CreateLocalStorage(ctx, s.Environment.BSVNetwork, s.ServerPrivateKey)
	}

	if err != nil {
		name := to.IfThen(remoteStorage, "remote").ElseThen("local")
		panic(fmt.Errorf("failed to create %s storage provider: %w", name, err))
	}

	userWallet, err := wallet.New(s.Environment.BSVNetwork, privkey, storageProvider)
	if err != nil {
		cleanup()
		panic(fmt.Errorf("failed to create wallet: %w", err))
	}

	slog.Info("CreateWallet", s.IdentityKey.ToDERHex())
	return userWallet, cleanup
}
