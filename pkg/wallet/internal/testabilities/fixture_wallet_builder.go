package testabilities

import (
	"log/slog"
	"net/http"
	"slices"
	"testing"

	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-resty/resty/v2"
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
	// StorageTypeRemote represents remote storage type based on SQLite using the V1 storage adapter protocol (/storage/v1/*).
	// The legacy JSON-RPC remote has been fully migrated to V1.
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
	WithNetwork(network defs.BSVNetwork) WalletBuilder
	ForUser(user testusers.User) *wallet.Wallet
	ForRootKey(rootKeyHex string) *wallet.Wallet
}

type walletBuilder struct {
	testing.TB

	walletFixture *walletFixture
	storageType   StorageType
	network       defs.BSVNetwork
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

// WithV1RemoteStorage explicitly requests the V1 adapter remote storage (same as WithRemoteStorage post-migration).
func (w *walletBuilder) WithV1RemoteStorage() WalletBuilder {
	return w.WithActiveStorage(StorageTypeRemote)
}

func (w *walletBuilder) WithSQLiteStorage() WalletBuilder {
	return w.WithActiveStorage(StorageTypeSQLite)
}

func (w *walletBuilder) WithMockedStorage() WalletBuilder {
	return w.WithActiveStorage(StorageTypeMocked)
}

func (w *walletBuilder) WithNetwork(network defs.BSVNetwork) WalletBuilder {
	w.network = network
	return w
}

func (w *walletBuilder) ForRootKey(rootKey string) *wallet.Wallet {
	w.TB.Helper()
	net := w.network
	if net == "" {
		net = defs.NetworkTestnet
	}
	if rootKey == "" {
		return w.ForUser(testusers.Alice)
	}
	priv, err := primitives.PrivateKeyFromHex(rootKey)
	require.NoError(w, err, "root_key must be valid hex private key for BRC-100 vector")
	keyDeriver := sdk.NewKeyDeriver(priv)
	activeStorage, cleanup := w.storageForRootKey()
	opts := slices.Clone(w.walletOpts)
	if w.withServices {
		serviceCfg := defs.DefaultServicesConfig(net)
		walletServices := services.New(slog.Default(), serviceCfg)
		opts = append(opts, wallet.WithServices(walletServices))
	}
	if w.client != nil {
		opts = append(opts, wallet.WithAuthHTTPClient(w.client))
	}
	userWallet, err := wallet.New(net, keyDeriver, activeStorage, opts...)
	require.NoErrorf(w, err, "Couldn't create wallet for root_key %s... - invalid test setup", rootKey[:min(8, len(rootKey))])
	w.walletFixture.addUserWalletSetup(&userWalletSetup{
		user:        testusers.User{Name: "brc100-" + rootKey[:min(8, len(rootKey))], ID: 999, PrivKey: rootKey},
		wallet:      userWallet,
		storage:     activeStorage,
		storageType: w.storageType,
		cleanupFunc: cleanup,
	})
	return userWallet
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (w *walletBuilder) storageForRootKey() (storage wdk.WalletStorageProvider, cleanup func()) {
	sqliteStorage := w.givenStorage.Provider().GORM()
	switch w.storageType {
	case StorageTypeSQLite, StorageTypeOwnSQLite, "":
		return sqliteStorage, nil
	case StorageTypeMocked:
		return w.givenStorage.MockProvider(), nil
	case StorageTypeRemote:
		// fallback for conformance vectors; remote requires user auth mapping
		w.Logf("BRC100 rootkey with remote not supported, falling back to sqlite storage")
		return sqliteStorage, nil
	default:
		w.Fatalf("invalid test setup for root key storage: %s", w.storageType)
		return nil, nil
	}
}

func (w *walletBuilder) ForUser(user testusers.User) *wallet.Wallet {
	privKey := user.PrivateKey(w)
	keyDeriver := sdk.NewKeyDeriver(privKey)
	activeStorage, cleanup := w.storageForUser(user)

	opts := slices.Clone(w.walletOpts)
	net := w.network
	if net == "" {
		net = defs.NetworkTestnet
	}
	if w.withServices {
		serviceCfg := defs.DefaultServicesConfig(net)
		transport := w.givenStorage.Provider().Transport()
		client := resty.New()
		client.SetTransport(transport)
		walletServices := services.New(slog.Default(), serviceCfg, services.WithRestyClient(client))
		opts = append(opts, wallet.WithServices(walletServices))
	}

	if w.client != nil {
		opts = append(opts, wallet.WithAuthHTTPClient(w.client))
	}

	userWallet, err := wallet.New(net, keyDeriver, activeStorage, opts...)
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
