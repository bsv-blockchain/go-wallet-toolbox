package testabilities

import (
	"context"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services"
	"log/slog"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/database"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/require"
)

type ProviderFixture interface {
	WithNetwork(network defs.BSVNetwork) ProviderFixture
	WithCommission(commission defs.Commission) ProviderFixture
	WithFeeModel(feeModel defs.FeeModel) ProviderFixture
	WithRandomizer(randomizer wdk.Randomizer) ProviderFixture
	WithServices() ProviderFixture

	GORM() *storage.Provider
	GORMWithCleanDatabase() *storage.Provider
}

type providerFixture struct {
	network    defs.BSVNetwork
	commission defs.Commission
	feeModel   defs.FeeModel
	randomizer wdk.Randomizer
	services   wdk.Services

	t               testing.TB
	require         *require.Assertions
	logger          *slog.Logger
	db              *database.Database
	activeStorage   *storage.Provider
	servicesFixture testabilities.ServicesFixture
}

func (p *providerFixture) WithNetwork(network defs.BSVNetwork) ProviderFixture {
	require.Nil(p.t, p.services, "please configure network before services")
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

func (p *providerFixture) WithServices() ProviderFixture {
	p.servicesFixture = testabilities.GivenServicesWithNetwork(p.t, p.network)

	p.servicesFixture.ARC().IsUpAndRunning()

	p.services = services.New(p.servicesFixture.ARC().HttpClient(), p.logger, p.network, defs.DefaultServicesConfig(p.network))
	return p
}

func (p *providerFixture) GORM() *storage.Provider {
	p.t.Helper()
	provider := p.GORMWithCleanDatabase()

	p.seedUsers(provider)

	return provider
}

func (p *providerFixture) GORMWithCleanDatabase() *storage.Provider {
	p.t.Helper()

	storageIdentityKey, err := wdk.IdentityKey(fixtures.StorageServerPrivKey)
	p.require.NoError(err)

	activeStorage, err := storage.NewGORMProvider(
		p.logger,
		storage.GORMProviderConfig{
			Chain:      p.network,
			FeeModel:   p.feeModel,
			Commission: p.commission,
			Services:   p.services,
		},
		storage.WithGORM(p.db.DB),
		storage.WithRandomizer(p.randomizer),
	)
	p.require.NoError(err)

	_, err = activeStorage.Migrate(context.Background(), fixtures.StorageName, storageIdentityKey)
	p.require.NoError(err)

	p.activeStorage = activeStorage

	return activeStorage
}

func (p *providerFixture) seedUsers(provider *storage.Provider) {
	for _, user := range testusers.All() {
		res, err := provider.FindOrInsertUser(context.Background(), user.PrivKey)
		p.require.NoError(err)

		user.ID = res.User.UserID
	}
}
