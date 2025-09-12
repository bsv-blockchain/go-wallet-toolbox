package testabilities

import (
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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

	StorageIdentityKey() string
}

type FaucetFixture interface {
	TopUp(satoshis satoshi.Value, opts ...TopUpOpts) (txtestabilities.TransactionSpec, *models.UserUTXO)
}

type TopUpOptions struct {
	Mined   bool
	Labels  []string
	Purpose string
}

type TopUpOpts = func(*TopUpOptions)

func WithMinedTopUp() TopUpOpts {
	return func(o *TopUpOptions) {
		o.Mined = true
	}
}

func WithLabelsTopUp(labels ...string) TopUpOpts {
	return func(o *TopUpOptions) {
		o.Labels = labels
	}
}

func WithPurpose(s string) TopUpOpts {
	return func(o *TopUpOptions) {
		o.Purpose = s
	}
}

type storageFixture struct {
	t          testing.TB
	require    *require.Assertions
	logger     *slog.Logger
	testServer *httptest.Server
	db         *database.Database

	providerFixture *providerFixture

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
	return s.providerFixture
}

func (s *storageFixture) Faucet(activeStorage *storage.Provider, user testusers.User) FaucetFixture {
	s.t.Helper()
	ctx := s.t.Context()

	_, err := activeStorage.FindOrInsertUser(ctx, user.PrivKey)
	s.require.NoError(err)

	basket, err := s.db.CreateRepositories().
		FindBasketByName(s.t.Context(), user.ID, wdk.BasketNameForChange)
	require.NoError(s.t, err)

	return &faucetFixture{
		t:          s.t,
		user:       user,
		db:         s.db,
		basketName: basket.Name,
	}
}

func (s *storageFixture) StorageIdentityKey() string {
	s.t.Helper()
	identityKey, err := wdk.IdentityKey(s.storagePrivKey)
	require.NoError(s.t, err)

	return identityKey
}

func Given(t testing.TB, configModifiers ...dbfixtures.DBConfigModifier) (given StorageFixture, cleanup func()) {
	return newStorageFixture(t, fixtures.StorageServerPrivKey, fixtures.StorageName, configModifiers...)
}

func GivenCustomStorage(t testing.TB, identityKey string, name string) (given StorageFixture, cleanup func()) {
	return newStorageFixture(t, identityKey, name, dbfixtures.WithSQLiteFileName(name))
}

func newStorageFixture(t testing.TB, identityKey string, name string, configModifiers ...dbfixtures.DBConfigModifier) (given StorageFixture, cleanup func()) {
	db, dbCleanup := dbfixtures.TestDatabase(t, configModifiers...)

	s := &storageFixture{
		t:              t,
		require:        require.New(t),
		logger:         logging.NewTestLogger(t),
		db:             db,
		storagePrivKey: identityKey,
		storageName:    name,
	}

	network := defs.NetworkTestnet

	servicesFixture := testservices.GivenServicesWithNetwork(t, network)

	servicesFixture.Services().New()

	s.providerFixture = &providerFixture{
		t:       s.t,
		require: s.require,
		logger:  s.logger,
		db:      s.db,

		ServicesFixture: servicesFixture,

		network:             network,
		commission:          defs.Commission{},
		feeModel:            defs.DefaultFeeModel(),
		failAbandoned:       defs.DefaultFailAbandoned(),
		randomizer:          randomizer.New(),
		beefVerifierFixture: newBeefVerifierFixture(),
		storagePrivKey:      s.storagePrivKey,
		storageName:         s.storageName,
	}

	return s, func() {
		s.providerFixture.Cleanup()
		dbCleanup()
	}
}
