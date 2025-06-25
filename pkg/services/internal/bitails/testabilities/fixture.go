package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/go-softwarelab/common/pkg/to"
)

type BitailsServiceFixture interface {
	testservices.ServicesFixture
	NewBitailsService() *bitails.Bitails
}

type bitailsServiceFixture struct {
	testservices.ServicesFixture
	t testing.TB
}

func Given(t testing.TB) BitailsServiceFixture {
	return &bitailsServiceFixture{
		ServicesFixture: testservices.GivenServices(t),
		t:               t,
	}
}

func (f *bitailsServiceFixture) NewBitailsService() *bitails.Bitails {
	logger := logging.NewTestLogger(f.t)
	httpClient := f.Bitails().HttpClient()
	network := f.Network()

	config := to.OptionsWithDefault(defs.Bitails{
		APIKey: "",
	})

	return bitails.New(httpClient, logger, network, config)
}
