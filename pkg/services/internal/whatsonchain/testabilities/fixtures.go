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
	NewWoCService() *whatsonchain.WhatsOnChain
}

type wocServiceFixture struct {
	testservices.ServicesFixture
	t testing.TB
}

func Given(t testing.TB) WoCServiceFixture {
	return &wocServiceFixture{
		ServicesFixture: testservices.GivenServices(t),
		t:               t,
	}
}

func (f *wocServiceFixture) NewWoCService() *whatsonchain.WhatsOnChain {
	logger := logging.NewTestLogger(f.t)
	httpClient := f.WhatsOnChain().HttpClient()
	network := f.Network()

	config := to.OptionsWithDefault(defs.WhatsOnChain{
		APIKey:            "",
		BSVExchangeRate:   defs.BSVExchangeRate{},
		BSVUpdateInterval: nil,
	})

	return whatsonchain.New(httpClient, logger, network, config)
}
