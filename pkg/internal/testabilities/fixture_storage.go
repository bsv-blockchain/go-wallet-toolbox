package testabilities

import (
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/bsv-blockchain/go-sdk/wallet"
	txtestabilities "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/slogx"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type StorageFixture interface {
	Provider() ProviderFixture

	// StartedRPCServerFor starts an in-memory HTTP test server whose Handler()
	// uses the conforming v1adapter (BRC-100 /storage/v1/* contract).
	// The legacy "RPC" name is kept for backward compatibility with existing tests;
	// under the hood it uses storage.Server + v1adapter, not the old jsonrpc layer.
	StartedRPCServerFor(provider wdk.WalletStorageProvider, opts ...func(options *storage.ServerOptions)) (cleanup func())
	// RPCClientForUser returns a remote storage client for the given user.
	// It now uses the V1 HTTP adapter protocol (deprecated "RPC" name retained for compat).
	// Prefer V1ClientForUser in new code.
	RPCClientForUser(user testusers.User) (*storage.WalletStorageProviderClient, func())

	// StartedV1AdapterServerFor is the explicit V1 name for starting the conforming server.
	StartedV1AdapterServerFor(provider wdk.WalletStorageProvider, opts ...func(options *storage.ServerOptions)) (cleanup func())
	// V1ClientForUser returns a client speaking the /storage/v1/* + auth contract.
	V1ClientForUser(user testusers.User) (*storage.WalletStorageProviderClient, func())

	ServerURL() string

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

func Given(t testing.TB, configModifiers ...dbfixtures.DBConfigModifier) (given StorageFixture, cleanup func()) {
	return newStorageFixture(t, fixtures.StorageServerPrivKey, fixtures.StorageName, configModifiers...)
}

func GivenCustomStorage(t testing.TB, identityKey, name string) (given StorageFixture, cleanup func()) {
	return newStorageFixture(t, identityKey, name, dbfixtures.WithSQLiteFileName(name))
}

func newStorageFixture(t testing.TB, identityKey, name string, configModifiers ...dbfixtures.DBConfigModifier) (given StorageFixture, cleanup func()) {
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

	s.providerFixture = &providerFixture{
		t:       s.t,
		require: s.require,
		logger:  s.logger,
		db:      s.db,

		ServicesFixture: servicesFixture,

		network:                network,
		commission:             defs.Commission{},
		feeModel:               defs.FeeModel{Type: defs.SatPerKB, Value: 1},
		failAbandoned:          defs.DefaultFailAbandoned(),
		syncTxStatuses:         defs.DefaultSynchronizeTxStatuses(),
		changeBasket:           defs.DefaultChangeBasket(),
		randomizer:             randomizer.New(),
		beefVerifierFixture:    newBeefVerifierFixture(),
		scriptsVerifierFixture: newScriptsVerifierFixture(),
		storagePrivKey:         s.storagePrivKey,
		storageName:            s.storageName,
	}

	return s, func() {
		s.providerFixture.Cleanup()
		dbCleanup()
	}
}

func (s *storageFixture) StartedRPCServerFor(provider wdk.WalletStorageProvider, opts ...func(*storage.ServerOptions)) (cleanup func()) {
	s.t.Helper()
	serverWallet := wallet.NewTestWallet(s.t, wallet.PrivHex(s.storagePrivKey), wallet.WithTestWalletLogger(s.logger))

	serverWallet.OnInternalizeAction().ReturnSuccess(&wallet.InternalizeActionResult{Accepted: true})

	serverOptions := to.OptionsWithDefault(storage.ServerOptions{}, opts...)

	storageServer := storage.NewServer(s.logger, provider, serverWallet, serverOptions)
	s.testServer = httptest.NewServer(storageServer.Handler())
	return s.testServer.Close
}

// StartedV1AdapterServerFor delegates to the V1 implementation (same as StartedRPCServerFor).
func (s *storageFixture) StartedV1AdapterServerFor(provider wdk.WalletStorageProvider, opts ...func(*storage.ServerOptions)) (cleanup func()) {
	return s.StartedRPCServerFor(provider, opts...)
}

func (s *storageFixture) RPCClientForUser(user testusers.User) (client *storage.WalletStorageProviderClient, cleanup func()) {
	s.t.Helper()
	protoWallet, err := wallet.NewCompletedProtoWallet(user.PrivateKey(s.t))
	s.require.NoErrorf(err, "Failed to create proto wallet for user %s", user.Name)

	client, cleanup, err = storage.NewClient(s.testServer.URL, protoWallet, storage.WithHttpClient(s.testServer.Client()), storage.WithClientLogger(slogx.NewTestLogger(s.t)))
	s.require.NoError(err)
	return client, cleanup
}

// V1ClientForUser is the explicit V1 name; delegates to the (now V1-based) client factory.
func (s *storageFixture) V1ClientForUser(user testusers.User) (client *storage.WalletStorageProviderClient, cleanup func()) {
	return s.RPCClientForUser(user)
}

// ServerURL returns the URL of the test server started by StartedRPCServerFor (now using the conforming v1adapter).
func (s *storageFixture) ServerURL() string {
	s.t.Helper()
	if s.testServer == nil {
		s.t.Fatal("Server not started — call StartedRPCServerFor first")
	}
	return s.testServer.URL
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
