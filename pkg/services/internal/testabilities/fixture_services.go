package testabilities

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/arc"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
)

type ServicesFixture interface {
	WhatsOnChain() WhatsOnChainFixture
	ARC() ARCFixture

	Services() WalletServicesFixture
	NewServicesWithConfig(config defs.WalletServices) *services.WalletServices
}

type WalletServicesFixture interface {
	WithDefaultConfig() *services.WalletServices

	WithBsvExchangeRate(exchangeRate defs.BSVExchangeRate) *services.WalletServices

	NewArcService(opts ...func(*arc.Config)) *arc.Service
}

type servicesFixture struct {
	t                    testing.TB
	require              *require.Assertions
	logger               *slog.Logger
	services             *services.WalletServices
	httpClient           *resty.Client
	transport            *httpmock.MockTransport
	walletServicesConfig *defs.WalletServices
	woc                  WhatsOnChainFixture
	arc                  ARCFixture
}

func Given(t testing.TB) ServicesFixture {
	transport := httpmock.NewMockTransport()
	client := resty.New()
	client.GetClient().Transport = transport

	servicesConfig := defs.DefaultServicesConfig(defs.NetworkTestnet)

	wocFx := NewWoCFixtureWithTransport(t, transport)
	arcFx := NewArcFixtureWithTransport(t, transport)

	return &servicesFixture{
		t:                    t,
		require:              require.New(t),
		logger:               logging.NewTestLogger(t),
		httpClient:           client,
		transport:            transport,
		walletServicesConfig: &servicesConfig,
		woc:                  wocFx,
		arc:                  arcFx,
	}
}

func (f *servicesFixture) WhatsOnChain() WhatsOnChainFixture {
	return f.woc
}

func (f *servicesFixture) ARC() ARCFixture {
	return f.arc
}

func (f *servicesFixture) WithDefaultConfig() *services.WalletServices {
	f.t.Helper()

	walletServices := services.New(f.httpClient, f.logger, *f.walletServicesConfig)
	f.services = walletServices

	return f.services
}

func (f *servicesFixture) WithBsvExchangeRate(exchangeRate defs.BSVExchangeRate) *services.WalletServices {
	f.t.Helper()
	f.walletServicesConfig.WhatsOnChain.BSVExchangeRate = exchangeRate

	walletServices := services.New(f.httpClient, f.logger, *f.walletServicesConfig)
	f.services = walletServices

	return f.services
}

func (f *servicesFixture) NewArcService(opts ...func(*arc.Config)) *arc.Service {
	logger := logging.NewTestLogger(f.t)
	httpClient := f.arc.HttpClient()
	config := to.OptionsWithDefault(arc.Config{
		URL:           ArcURL,
		Token:         ArcToken,
		DeploymentID:  DeploymentID,
		WaitFor:       "",
		CallbackURL:   "",
		CallbackToken: "",
	}, opts...)

	return arc.NewARCService(logger, httpClient, config)
}

func (f *servicesFixture) Services() WalletServicesFixture {
	return f
}

func (f *servicesFixture) NewServicesWithConfig(config defs.WalletServices) *services.WalletServices {
	f.t.Helper()

	walletServices := services.New(f.httpClient, f.logger, config)

	f.services = walletServices

	return f.services
}
