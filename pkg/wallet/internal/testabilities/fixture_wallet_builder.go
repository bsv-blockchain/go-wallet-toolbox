package testabilities

import (
	"log/slog"
	"net/http"
	"slices"
	"testing"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type StorageType string

const (
	// StorageTypeSQLite represents SQLite storage type.
	StorageTypeSQLite StorageType = "sqlite"
	// StorageTypeRemote represents remote storage type based on SQLite.
	StorageTypeRemote StorageType = "remote"
	// StorageTypeMocked represents a mocked storage type.
	StorageTypeMocked StorageType = "mocked"
	// StorageTypeOwnSQLite represents a separate SQLite storage type.
	StorageTypeOwnSQLite StorageType = "own_sqlite"
)

type WalletBuilder interface {
	WithActiveStorage(storageType StorageType) WalletBuilder
	WithRemoteStorage() WalletBuilder
	WithSQLiteStorage() WalletBuilder
	WithServices() WalletBuilder
	WithOwnStorage() WalletBuilder
	WithHTTPClient(client *http.Client) WalletBuilder
	WithWalletOpts(opts ...func(*wallet_opts.Opts)) WalletBuilder
	ForUser(user testusers.User) *wallet.Wallet
}

type walletBuilder struct {
	testing.TB

	walletFixture *walletFixture
	storageType   StorageType
	withServices  bool
	givenStorage  testabilities.StorageFixture
	walletOpts    []func(*wallet_opts.Opts)
	client        *http.Client
}

func (w *walletBuilder) WithOwnStorage() WalletBuilder {
	return w.WithActiveStorage(StorageTypeOwnSQLite)
}

func (w *walletBuilder) WithHTTPClient(client *http.Client) WalletBuilder {
	w.client = client
	return w
}

func (w *walletBuilder) WithActiveStorage(storageType StorageType) WalletBuilder {
	w.storageType = storageType
	return w
}

func (w *walletBuilder) WithServices() WalletBuilder {
	w.withServices = true
	return w
}

func (w *walletBuilder) WithWalletOpts(opts ...func(*wallet_opts.Opts)) WalletBuilder {
	w.walletOpts = append(w.walletOpts, opts...)
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
	activeStorage, cleanup := w.storageForUser(user)

	opts := slices.Clone(w.walletOpts)
	if w.withServices {
		serviceCfg := defs.DefaultServicesConfig(defs.NetworkTestnet)
		walletServices := services.New(slog.Default(), serviceCfg)
		opts = append(opts, wallet.WithServices(walletServices))
	}

	if w.client != nil {
		opts = append(opts, wallet.WithAuthHTTPClient(w.client))
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

func (w *walletBuilder) storageForUser(user testusers.User) (storage wdk.WalletStorageProvider, cleanup func()) {
	sqliteStorage := w.givenStorage.Provider().GORM()
	switch w.storageType {
	case StorageTypeSQLite:
		return sqliteStorage, nil
	case StorageTypeOwnSQLite:
		given, cleanupFunc := testabilities.GivenCustomStorage(w, fixtures.SecondStorageIdentityKey, user.Name)
		return given.Provider().GORM(), cleanupFunc
	case StorageTypeRemote:
		serverCleanup := w.givenStorage.StartedRPCServerFor(sqliteStorage)
		storageClient, clientCleanup := w.givenStorage.RPCClientForUser(user)
		return storageClient, func() {
			clientCleanup()
			serverCleanup()
		}
	case StorageTypeMocked:
		return w.givenStorage.MockProvider(), nil
	default:
		w.Fatalf("invalid test setup: not implemented support for storage type: %s", w.storageType)
		return storage, cleanup
	}
}
