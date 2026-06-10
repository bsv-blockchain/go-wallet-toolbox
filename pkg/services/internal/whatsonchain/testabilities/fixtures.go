package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
)

type WoCServiceFixture interface {
	testservices.ServicesFixture

	NewWoCService(opts ...func(*whatsonchain.WhatsOnChain)) *whatsonchain.WhatsOnChain
}

func Given(t testing.TB) WoCServiceFixture {
	return &wocServiceFixture{
		ServicesFixture: testservices.GivenServices(t),
		t:               t,
	}
}

// WithRequestsPerSecond reconfigures the client-side rate limiter of the WoC service under test.
func WithRequestsPerSecond(requestsPerSecond float64) func(*whatsonchain.WhatsOnChain) {
	return func(service *whatsonchain.WhatsOnChain) {
		service.SetRequestsPerSecond(requestsPerSecond)
	}
}

type wocServiceFixture struct {
	testservices.ServicesFixture

	t testing.TB
}

func (f *wocServiceFixture) NewWoCService(opts ...func(*whatsonchain.WhatsOnChain)) *whatsonchain.WhatsOnChain {
	logger := logging.NewTestLogger(f.t)
	client := f.WhatsOnChain().HttpClient()
	network := f.Network()

	config := defs.WhatsOnChain{
		BSVExchangeRate:            defs.BSVExchangeRate{},
		RootForHeightRetryInterval: 0,
		RootForHeightRetries:       1,
		// NOTE: tests should not be slowed down by the client-side WoC rate limiter
		RequestsPerSecond: 10000,
	}

	service := whatsonchain.New(client, logger, network, config)

	for _, opt := range opts {
		opt(service)
	}

	return service
}
