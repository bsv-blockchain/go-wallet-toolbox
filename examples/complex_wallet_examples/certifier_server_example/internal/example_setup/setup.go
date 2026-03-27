package example_setup

import (
	"context"
	"fmt"
	"log/slog"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

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
	remoteStorage := s.Environment.ServerURL != ""

	userWallet, err := wallet.NewWithStorageFactory(s.Environment.BSVNetwork, privkey, func(userWallet sdk.Interface) (wdk.WalletStorageProvider, func(), error) {
		if remoteStorage {
			slog.Info("Using remote storage", "url", s.Environment.ServerURL)
			return storage.NewClient(s.Environment.ServerURL, userWallet)
		}
		slog.Info("Using local storage", "file", SQLiteStorageFile)
		return CreateLocalStorage(ctx, s.Environment.BSVNetwork, s.ServerPrivateKey)
	})
	if err != nil {
		panic(fmt.Errorf("failed to create wallet: %w", err))
	}

	slog.Info("CreateWallet", "identityKey", s.IdentityKey.ToDERHex())
	return userWallet, userWallet.Close
}
