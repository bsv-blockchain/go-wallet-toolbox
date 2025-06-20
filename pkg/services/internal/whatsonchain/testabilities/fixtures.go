package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/go-softwarelab/common/pkg/to"
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

type wocServiceFixture struct {
	testservices.ServicesFixture
	t testing.TB
}

func (f *wocServiceFixture) NewWoCService(opts ...func(*whatsonchain.WhatsOnChain)) *whatsonchain.WhatsOnChain {
	logger := logging.NewTestLogger(f.t)
	client := f.WhatsOnChain().HttpClient()
	network := f.Network()

	config := to.OptionsWithDefault(defs.WhatsOnChain{
		BSVExchangeRate: defs.BSVExchangeRate{},
	})

	service := whatsonchain.New(client, logger, network, config)

	for _, opt := range opts {
		opt(service)
	}

	return service
}
