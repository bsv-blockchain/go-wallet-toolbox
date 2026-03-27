package testservices

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
)

const headerLength = 160

var TestFakeHeaderBinary = mockHeaderBinary('0')

type ServicesFixture interface {
	Bitails() BitailsFixture
	WhatsOnChain() WhatsOnChainFixture
	ARC() ARCFixture
	BHS() BHSFixture
	Chaintracks() ChaintracksClientFixture
	Services() WalletServicesFixture

	Network() defs.BSVNetwork
	Transport() *httpmock.MockTransport
}

type WalletServicesFixture interface {
	Config(modifiers ...func(*defs.WalletServices)) WalletServicesFixture
	Opts(options ...func(option *services.Options)) WalletServicesFixture
	New() *services.WalletServices
}

type servicesFixture struct {
	t                    testing.TB
	require              *require.Assertions
	logger               *slog.Logger
	services             *services.WalletServices
	httpClient           *resty.Client
	transport            *httpmock.MockTransport
	walletServicesConfig *defs.WalletServices
	walletServicesOpts   []func(option *services.Options)
	woc                  WhatsOnChainFixture
	arc                  ARCFixture
	bitails              BitailsFixture
	bhs                  BHSFixture
	network              defs.BSVNetwork
	chaintracksClient    ChaintracksClientFixture
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
	client.SetTransport(transport)

	servicesConfig := defs.DefaultServicesConfig(network)
	servicesConfig.WhatsOnChain.RootForHeightRetries = 1
	servicesConfig.WhatsOnChain.RootForHeightRetryInterval = 0

	wocFx := NewWoCFixture(t, WithTransport(transport), WithNetwork(network))
	arcFx := NewARCFixture(t, WithTransport(transport), WithNetwork(network))
	bitailsFx := NewBitailsFixture(t, WithTransport(transport), WithNetwork(network))
	bhsFx := NewBHSFixture(t, WithTransport(transport))
	chaintracksClient := NewChaintracksClientFixture(t)

	return &servicesFixture{
		t:                    t,
		require:              require.New(t),
		logger:               logging.NewTestLogger(t),
		httpClient:           client,
		transport:            transport,
		walletServicesConfig: &servicesConfig,
		network:              network,
		woc:                  wocFx,
		bhs:                  bhsFx,
		arc:                  arcFx,
		bitails:              bitailsFx,
		chaintracksClient:    chaintracksClient,
	}
}

func (f *servicesFixture) WhatsOnChain() WhatsOnChainFixture {
	return f.woc
}

func (f *servicesFixture) ARC() ARCFixture {
	return f.arc
}

func (f *servicesFixture) Chaintracks() ChaintracksClientFixture {
	return f.chaintracksClient
}

func (f *servicesFixture) Bitails() BitailsFixture {
	return f.bitails
}

func (f *servicesFixture) BHS() BHSFixture {
	return f.bhs
}

func (f *servicesFixture) Config(modifiers ...func(*defs.WalletServices)) WalletServicesFixture {
	f.t.Helper()

	for _, modify := range modifiers {
		modify(f.walletServicesConfig)
	}

	return f
}

func (f *servicesFixture) Opts(options ...func(option *services.Options)) WalletServicesFixture {
	f.t.Helper()
	f.walletServicesOpts = append(f.walletServicesOpts, options...)
	return f
}

func (f *servicesFixture) New() *services.WalletServices {
	f.t.Helper()

	options := append(f.walletServicesOpts,
		services.WithRestyClient(f.httpClient),
		services.WithChaintracksAdapter(f.chaintracksClient.Adapter()),
	)

	walletServices := services.New(f.logger, *f.walletServicesConfig, options...)
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

func mockHeaderBinary(char rune) string {
	return strings.Repeat(string(char), headerLength)
}

func WithBsvExchangeRate(exchangeRate defs.BSVExchangeRate) func(*defs.WalletServices) {
	return func(ws *defs.WalletServices) {
		ws.WhatsOnChain.BSVExchangeRate = exchangeRate
	}
}

func WithEnabledBitails(enabled bool) func(*defs.WalletServices) {
	return func(ws *defs.WalletServices) {
		ws.Bitails.Enabled = enabled
	}
}

func WithEnabledBHS(enabled bool) func(*defs.WalletServices) {
	return func(ws *defs.WalletServices) {
		ws.BHS.Enabled = enabled
	}
}

func WithEnabledARC(enabled bool) func(*defs.WalletServices) {
	return func(ws *defs.WalletServices) {
		ws.ArcConfig.Enabled = enabled
	}
}

func WithEnabledWoC(enabled bool) func(*defs.WalletServices) {
	return func(ws *defs.WalletServices) {
		ws.WhatsOnChain.Enabled = enabled
	}
}

func WithEnabledChaintracks(enabled bool) func(*defs.WalletServices) {
	return func(ws *defs.WalletServices) {
		ws.ChaintracksClient.Enabled = enabled
	}
}
