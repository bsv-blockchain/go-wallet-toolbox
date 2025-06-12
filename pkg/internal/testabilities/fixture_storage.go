package testabilities

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type StorageFixture interface {
	Provider() ProviderFixture

	StartedRPCServerFor(provider wdk.WalletStorageProvider) (cleanup func())
	RPCClient() (*storage.WalletStorageProviderClient, func())

	MockProvider() *mocks.MockWalletStorageProvider

	Faucet(activeStorage *storage.Provider, user testusers.User) FaucetFixture
}

type FaucetFixture interface {
	TopUp(satoshis satoshi.Value) (txtestabilities.TransactionSpec, *models.UserUTXO)
}

type storageFixture struct {
	t          testing.TB
	require    *require.Assertions
	logger     *slog.Logger
	testServer *httptest.Server
	db         *database.Database

	storagePrivKey string
	storageName    string
}

func (s *storageFixture) StartedRPCServerFor(provider wdk.WalletStorageProvider) (cleanup func()) {
	s.t.Helper()
	storageServer := storage.NewServer(s.logger, provider, storage.ServerOptions{})
	s.testServer = httptest.NewServer(storageServer.Handler())
	return s.testServer.Close
}

func (s *storageFixture) RPCClient() (client *storage.WalletStorageProviderClient, cleanup func()) {
	s.t.Helper()
	client, cleanup, err := storage.NewClient(s.testServer.URL, storage.WithHttpClient(s.testServer.Client()))
	s.require.NoError(err)
	return client, cleanup
}

func (s *storageFixture) MockProvider() *mocks.MockWalletStorageProvider {
	s.t.Helper()
	ctrl := gomock.NewController(s.t)

	return mocks.NewMockWalletStorageProvider(ctrl)
}

func (s *storageFixture) Provider() ProviderFixture {
	s.t.Helper()
	return &providerFixture{
		t:       s.t,
		require: s.require,
		logger:  s.logger,
		db:      s.db,

		network:        defs.NetworkTestnet,
		commission:     defs.Commission{},
		feeModel:       defs.DefaultFeeModel(),
		randomizer:     randomizer.New(),
		storagePrivKey: s.storagePrivKey,
		storageName:    s.storageName,
	}
}

func (s *storageFixture) Faucet(activeStorage *storage.Provider, user testusers.User) FaucetFixture {
	s.t.Helper()
	ctx := s.t.Context()

	_, err := activeStorage.FindOrInsertUser(ctx, user.PrivKey)
	s.require.NoError(err)

	basket, err := s.db.CreateRepositories().
		FindBasketByName(context.Background(), user.ID, wdk.BasketNameForChange)
	require.NoError(s.t, err)

	return &faucetFixture{
		t:          s.t,
		user:       user,
		db:         s.db,
		basketName: basket.Name,
	}
}

func Given(t testing.TB, configModifiers ...dbfixtures.DBConfigModifier) (given StorageFixture, cleanup func()) {
	db, dbCleanup := dbfixtures.TestDatabase(t, configModifiers...)
	return &storageFixture{
		t:              t,
		require:        require.New(t),
		logger:         logging.NewTestLogger(t),
		db:             db,
		storagePrivKey: fixtures.StorageServerPrivKey,
		storageName:    fixtures.StorageName,
	}, dbCleanup
}

func GivenCustomStorage(t testing.TB, identityKey string, name string) (given StorageFixture, cleanup func()) {
	db, dbCleanup := dbfixtures.TestDatabase(t, dbfixtures.WithSQLiteFileName(name))
	return &storageFixture{
		t:              t,
		require:        require.New(t),
		logger:         logging.NewTestLogger(t),
		db:             db,
		storagePrivKey: identityKey,
		storageName:    name,
	}, dbCleanup
}
