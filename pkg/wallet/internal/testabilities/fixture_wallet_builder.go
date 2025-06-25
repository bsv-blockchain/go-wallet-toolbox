package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
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
	ForUser(user testusers.User) *wallet.Wallet
}

type walletBuilder struct {
	testing.TB
	walletFixture *walletFixture
	storageType   StorageType
	givenStorage  testabilities.StorageFixture
}

func (w *walletBuilder) WithActiveStorage(storageType StorageType) WalletBuilder {
	w.storageType = storageType
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

	userWallet, err := wallet.New(defs.NetworkTestnet, keyDeriver, activeStorage)
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
