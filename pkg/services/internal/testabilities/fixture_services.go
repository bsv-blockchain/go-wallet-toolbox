package testabilities

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/configuration"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/arc"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-resty/resty/v2"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

type ServicesFixture interface {
	WhatsOnChain() WhatsOnChainFixture
	ARC() ARCFixture

	Services() WalletServicesFixture
	NewServicesWithConfig(config configuration.WalletServices) *services.WalletServices
}

type WalletServicesFixture interface {
	WithDefaultConfig() *services.WalletServices

	WithBsvExchangeRate(exchangeRate wdk.BSVExchangeRate) *services.WalletServices

	NewArcService(opts ...func(*arc.Config)) *arc.Service
}

type servicesFixture struct {
	t                    testing.TB
	require              *require.Assertions
	logger               *slog.Logger
	services             *services.WalletServices
	httpClient           *resty.Client
	transport            *httpmock.MockTransport
	walletServicesConfig *configuration.WalletServices
	woc                  WhatsOnChainFixture
	arc                  ARCFixture
}

func Given(t testing.TB) ServicesFixture {
	transport := httpmock.NewMockTransport()
	client := resty.New()
	client.GetClient().Transport = transport

	servicesConfig := servicesCfg(defs.NetworkTestnet)

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

func (f *servicesFixture) WithBsvExchangeRate(exchangeRate wdk.BSVExchangeRate) *services.WalletServices {
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

func (f *servicesFixture) NewServicesWithConfig(config configuration.WalletServices) *services.WalletServices {
	f.t.Helper()

	walletServices := services.New(f.httpClient, f.logger, config)

	f.services = walletServices

	return f.services
}

func servicesCfg(chain defs.BSVNetwork) configuration.WalletServices {
	var taalApiKey string
	var port int
	var arcUrl string

	if chain == defs.NetworkMainnet {
		taalApiKey = "mainnet_9596de07e92300c6287e4393594ae39c"
		port = 8084
		arcUrl = "https://api.taal.com/arc"
	} else {
		taalApiKey = "testnet_0e6cf72133b43ea2d7861da2a38684e3"
		port = 8083
		arcUrl = "https://arc-test.taal.com/arc"
	}

	return configuration.WalletServices{
		Chain:      chain,
		TaalAPIKey: taalApiKey,
		WhatsOnChain: configuration.WhatsOnChain{
			BSVUpdateInterval: to.Ptr(whatsonchain.DefaultBSVExchangeUpdateInterval),
		},
		FiatExchangeRates: wdk.FiatExchangeRates{
			Timestamp: time.Date(2023, time.December, 13, 0, 0, 0, 0, time.UTC),
			Base:      wdk.USD,
			Rates: map[wdk.Currency]float64{
				wdk.USD: 1,
				wdk.GBP: 0.8,
				wdk.EUR: 0.93,
			},
		},
		FiatUpdateInterval:              to.Ptr(whatsonchain.DefaultFiatExchangeUpdateInterval),
		DisableMapiCallback:             true, // rely on WalletMonitor by default
		ExchangeratesApiKey:             "bd539d2ff492bcb5619d5f27726a766f",
		ChaintracksFiatExchangeRatesUrl: fmt.Sprintf("https://npm-registry.babbage.systems:%d/getFiatExchangeRates", port),
		Chaintracks:                     nil, // TODO: implement me
		ArcURL:                          arcUrl,
		ArcConfig:                       nil, // TODO: implement me
	}
}
