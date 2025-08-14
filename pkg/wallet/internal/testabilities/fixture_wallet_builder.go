package testabilities

import (
	"log/slog"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

type StorageType string

const (
	// StorageTypeSQLite represents SQLite storage type.
	StorageTypeSQLite StorageType = "sqlite"
	// StorageTypeRemote represents remote storage type based on SQLite.
	StorageTypeRemote StorageType = "remote"
	// StorageTypeMocked represents a mocked storage type.
	StorageTypeMocked StorageType = "mocked"
)

type WalletBuilder interface {
	WithActiveStorage(storageType StorageType) WalletBuilder
	WithRemoteStorage() WalletBuilder
	WithSQLiteStorage() WalletBuilder
	WithServices() WalletBuilder
	ForUser(user testusers.User) *wallet.Wallet
}

type walletBuilder struct {
	testing.TB
	walletFixture *walletFixture
	storageType   StorageType
	withServices  bool
	givenStorage  testabilities.StorageFixture
}

func (w *walletBuilder) WithActiveStorage(storageType StorageType) WalletBuilder {
	w.storageType = storageType
	return w
}

func (w *walletBuilder) WithServices() WalletBuilder {
	w.withServices = true
	return w
}

func (w *walletBuilder) WithRemoteStorage() WalletBuilder {
	return w.WithActiveStorage(StorageTypeRemote)
}

func (w *walletBuilder) WithSQLiteStorage() WalletBuilder {
	return w.WithActiveStorage(StorageTypeSQLite)
}

func (w *walletBuilder) WithMockedStorage() WalletBuilder {
	return w.WithActiveStorage(StorageTypeMocked)
}

func (w *walletBuilder) ForUser(user testusers.User) *wallet.Wallet {
	privKey := user.PrivateKey(w)
	keyDeriver := sdk.NewKeyDeriver(privKey)
	activeStorage, cleanup := w.storage()

	var opts []func(*wallet_opts.Opts)
	if w.withServices {
		serviceCfg := defs.DefaultServicesConfig(defs.NetworkTestnet)
		walletServices := services.New(slog.Default(), serviceCfg)
		opts = append(opts, wallet.WithServices(walletServices))
	}

	userWallet, err := wallet.New(defs.NetworkTestnet, keyDeriver, activeStorage, opts...)
	require.NoErrorf(w, err, "Couldn't create wallet for user %s - invalid test setup", user.Name)

	w.walletFixture.addUserWalletSetup(&userWalletSetup{
		user:        user,
		wallet:      userWallet,
		storage:     activeStorage,
		storageType: w.storageType,
		cleanupFunc: cleanup,
	})

	return userWallet
}

func (w *walletBuilder) storage() (storage wdk.WalletStorageProvider, cleanup func()) {
	sqliteStorage := w.givenStorage.Provider().GORM()
	switch w.storageType {
	case StorageTypeSQLite:
		return sqliteStorage, nil
	case StorageTypeRemote:
		serverCleanup := w.givenStorage.StartedRPCServerFor(sqliteStorage)
		storageClient, clientCleanup := w.givenStorage.RPCClient()
		return storageClient, func() {
			clientCleanup()
			serverCleanup()
		}
	case StorageTypeMocked:
		return w.givenStorage.MockProvider(), nil
	default:
		w.Fatalf("invalid test setup: not implemented support for storage type: %s", w.storageType)
		return
	}
}
