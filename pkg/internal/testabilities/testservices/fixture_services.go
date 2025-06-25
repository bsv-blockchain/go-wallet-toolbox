package testservices

import (
	"log/slog"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services"
	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

type ServicesFixture interface {
	Bitails() BitailsFixture
	WhatsOnChain() WhatsOnChainFixture
	ARC() ARCFixture

	Services() WalletServicesFixture

	Network() defs.BSVNetwork
	Transport() *httpmock.MockTransport
}

type WalletServicesFixture interface {
	WithDefaultConfig() *services.WalletServices
	WithBsvExchangeRate(exchangeRate defs.BSVExchangeRate) *services.WalletServices
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
	bitails              BitailsFixture
	network              defs.BSVNetwork
}

func GivenServices(t testing.TB) ServicesFixture {
	network := defs.NetworkMainnet
	return givenServicesWithNetwork(t, network)
}

func GivenServicesWithNetwork(t testing.TB, network defs.BSVNetwork) ServicesFixture {
	return givenServicesWithNetwork(t, network)
}

func givenServicesWithNetwork(t testing.TB, network defs.BSVNetwork) ServicesFixture {
	transport := httpmock.NewMockTransport()
	client := resty.New()
	client.GetClient().Transport = transport

	servicesConfig := defs.DefaultServicesConfig(network)
	servicesConfig.WhatsOnChain.BroadcastDelay = 0

	wocFx := NewWoCFixture(t, WithTransport(transport), WithNetwork(network))
	arcFx := NewARCFixture(t, WithTransport(transport), WithNetwork(network))
	bitailsFx := NewBitailsFixture(t, WithTransport(transport), WithNetwork(network))

	return &servicesFixture{
		t:                    t,
		require:              require.New(t),
		logger:               logging.NewTestLogger(t),
		httpClient:           client,
		transport:            transport,
		walletServicesConfig: &servicesConfig,
		network:              network,
		woc:                  wocFx,
		arc:                  arcFx,
		bitails:              bitailsFx,
	}
}

func (f *servicesFixture) WhatsOnChain() WhatsOnChainFixture {
	return f.woc
}

func (f *servicesFixture) ARC() ARCFixture {
	return f.arc
}

func (f *servicesFixture) Bitails() BitailsFixture {
	return f.bitails
}

func (f *servicesFixture) WithDefaultConfig() *services.WalletServices {
	f.t.Helper()

	walletServices := services.New(f.logger, *f.walletServicesConfig, services.WithRestyClient(f.httpClient))
	f.services = walletServices

	return f.services
}

func (f *servicesFixture) WithBsvExchangeRate(exchangeRate defs.BSVExchangeRate) *services.WalletServices {
	f.t.Helper()
	f.walletServicesConfig.WhatsOnChain.BSVExchangeRate = exchangeRate

	walletServices := services.New(f.logger, *f.walletServicesConfig, services.WithRestyClient(f.httpClient))
	f.services = walletServices

	return f.services
}

func (f *servicesFixture) Services() WalletServicesFixture {
	return f
}

func (f *servicesFixture) NewServicesWithConfig(config defs.WalletServices) *services.WalletServices {
	f.t.Helper()

	walletServices := services.New(f.logger, config, services.WithRestyClient(f.httpClient))

	f.services = walletServices

	return f.services
}

func (f *servicesFixture) Network() defs.BSVNetwork {
	return f.network
}

func (f *servicesFixture) Transport() *httpmock.MockTransport {
	f.t.Helper()
	require.NotNil(f.t, f.transport, "Transport() called without setting up services fixture")
	return f.transport
}
