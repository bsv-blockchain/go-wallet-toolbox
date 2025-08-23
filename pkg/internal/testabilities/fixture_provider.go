package testabilities

import (
	"log/slog"
	"testing"

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
	WithFailAbandonedMinTxAge(seconds uint) ProviderFixture

	GORM() *storage.Provider
	GORMWithCleanDatabase() *storage.Provider

	StorageIdentityKey() string

	BeefVerifier() BeefVerifierFixture
}

type providerFixture struct {
	testservices.ServicesFixture

	network        defs.BSVNetwork
	commission     defs.Commission
	feeModel       defs.FeeModel
	failAbandoned  defs.FailAbandoned
	randomizer     wdk.Randomizer
	services       wdk.Services
	beefVerifier   *beefVerifierFixture
	storagePrivKey string
	storageName    string
	providers      []*storage.Provider

	t               testing.TB
	require         *require.Assertions
	logger          *slog.Logger
	db              *database.Database
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

func (p *providerFixture) WithFailAbandonedMinTxAge(seconds uint) ProviderFixture {
	p.failAbandoned = defs.FailAbandoned{
		MinTransactionAgeSeconds: seconds,
	}
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
		p.network,
		defs.Database{}, // Database connection is provided via option
		p.services,
		storage.WithGORM(p.db.DB),
		storage.WithRandomizer(p.randomizer),
		storage.WithBeefVerifier(p.beefVerifier),
		storage.WithFeeModel(p.feeModel),
		storage.WithCommission(p.commission),
		storage.WithFailAbandoned(p.failAbandoned),
	)
	p.require.NoError(err)

	_, err = activeStorage.Migrate(p.t.Context(), p.storageName, storageIdentityKey)
	p.require.NoError(err)

	p.providers = append(p.providers, activeStorage)

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

func (p *providerFixture) Cleanup() {
	p.t.Helper()
	for _, provider := range p.providers {
		provider.Stop()
	}
}

func (p *providerFixture) BeefVerifier() BeefVerifierFixture {
	p.t.Helper()
	return p.beefVerifier
}
