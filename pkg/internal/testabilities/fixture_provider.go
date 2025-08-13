package testabilities

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
)

type ServicesFixture interface {
	Bitails() testservices.BitailsFixture
	WhatsOnChain() testservices.WhatsOnChainFixture
	ARC() testservices.ARCFixture
	BHS() testservices.BHSFixture
	ServicesSniffer() *testutils.HTTPSniffer
	Transport() *httpmock.MockTransport
}

type ProviderFixture interface {
	ServicesFixture

	WithNetwork(network defs.BSVNetwork) ProviderFixture
	WithCommission(commission defs.Commission) ProviderFixture
	WithFeeModel(feeModel defs.FeeModel) ProviderFixture
	WithRandomizer(randomizer wdk.Randomizer) ProviderFixture

	GORM() *storage.Provider
	GORMWithCleanDatabase() *storage.Provider

	StorageIdentityKey() string
}

type providerFixture struct {
	testservices.ServicesFixture

	network        defs.BSVNetwork
	commission     defs.Commission
	feeModel       defs.FeeModel
	randomizer     wdk.Randomizer
	services       wdk.Services
	storagePrivKey string
	storageName    string

	t               testing.TB
	require         *require.Assertions
	logger          *slog.Logger
	db              *database.Database
	activeStorage   *storage.Provider
	servicesSniffer *testutils.HTTPSniffer
}

func (p *providerFixture) WithNetwork(network defs.BSVNetwork) ProviderFixture {
	p.network = network
	return p
}

func (p *providerFixture) WithCommission(commission defs.Commission) ProviderFixture {
	p.commission = commission
	return p
}

func (p *providerFixture) WithFeeModel(feeModel defs.FeeModel) ProviderFixture {
	p.feeModel = feeModel
	return p
}

func (p *providerFixture) WithRandomizer(randomizer wdk.Randomizer) ProviderFixture {
	p.randomizer = randomizer
	return p
}

func (p *providerFixture) withServices() ProviderFixture {
	p.ARC().IsUpAndRunning()

	mockTransport := p.Transport()
	p.servicesSniffer = testutils.NewHTTPSniffer(mockTransport)
	client := resty.New()
	client.SetTransport(p.servicesSniffer)

	p.services = services.New(p.logger, defs.DefaultServicesConfig(p.network), services.WithRestyClient(client))
	return p
}

func (p *providerFixture) ServicesSniffer() *testutils.HTTPSniffer {
	p.t.Helper()
	require.NotNil(p.t, p.servicesSniffer, "Sniffer() called without setting up services fixture")
	return p.servicesSniffer
}

func (p *providerFixture) GORM() *storage.Provider {
	p.t.Helper()
	provider := p.GORMWithCleanDatabase()

	p.seedUsers(provider)

	return provider
}

func (p *providerFixture) GORMWithCleanDatabase() *storage.Provider {
	p.t.Helper()
	p.withServices()

	storageIdentityKey, err := wdk.IdentityKey(p.storagePrivKey)
	p.require.NoError(err)

	activeStorage, err := storage.NewGORMProvider(
		p.t.Context(),
		p.logger,
		storage.GORMProviderConfig{
			Chain:                 p.network,
			FeeModel:              p.feeModel,
			Commission:            p.commission,
			Services:              p.services,
			SynchronizeTxStatuses: defs.DefaultSynchronizeTxStatuses(),
		},
		storage.WithGORM(p.db.DB),
		storage.WithRandomizer(p.randomizer),
	)
	p.require.NoError(err)

	_, err = activeStorage.Migrate(p.t.Context(), p.storageName, storageIdentityKey)
	p.require.NoError(err)

	p.activeStorage = activeStorage

	return activeStorage
}

func (p *providerFixture) StorageIdentityKey() string {
	p.t.Helper()
	identityKey, err := wdk.IdentityKey(p.storagePrivKey)
	require.NoError(p.t, err)

	return identityKey
}

func (p *providerFixture) seedUsers(provider *storage.Provider) {
	for _, user := range testusers.All() {
		res, err := provider.FindOrInsertUser(p.t.Context(), user.IdentityKey(p.t))
		p.require.NoError(err)

		user.ID = res.User.UserID
	}
}
