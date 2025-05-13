package testabilities

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
	WhatsOnChain() WhatsOnChainFixture
	ARC() ARCFixture

	Services() WalletServicesFixture

	Network() defs.BSVNetwork
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
	network              defs.BSVNetwork
}

func Given(t testing.TB) ServicesFixture {
	transport := httpmock.NewMockTransport()
	client := resty.New()
	client.GetClient().Transport = transport

	network := defs.NetworkMainnet
	servicesConfig := defs.DefaultServicesConfig(network)

	wocFx := NewWoCFixture(t, WithTransport(transport), WithNetwork(network))
	arcFx := NewARCFixture(t, WithTransport(transport), WithNetwork(network))

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

	walletServices := services.New(f.httpClient, f.logger, f.network, *f.walletServicesConfig)
	f.services = walletServices

	return f.services
}

func (f *servicesFixture) WithBsvExchangeRate(exchangeRate defs.BSVExchangeRate) *services.WalletServices {
	f.t.Helper()
	f.walletServicesConfig.WhatsOnChain.BSVExchangeRate = exchangeRate

	walletServices := services.New(f.httpClient, f.logger, f.network, *f.walletServicesConfig)
	f.services = walletServices

	return f.services
}

func (f *servicesFixture) Services() WalletServicesFixture {
	return f
}

func (f *servicesFixture) NewServicesWithConfig(config defs.WalletServices) *services.WalletServices {
	f.t.Helper()

	walletServices := services.New(f.httpClient, f.logger, f.network, config)

	f.services = walletServices

	return f.services
}

func (f *servicesFixture) Network() defs.BSVNetwork {
	return f.network
}
