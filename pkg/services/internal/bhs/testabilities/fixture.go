package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bhs"
)

type BHSServiceFixture interface {
	testservices.ServicesFixture
	NewBHSService() *bhs.BlockHeadersService
}

type bhsServiceFixture struct {
	testservices.ServicesFixture

	t testing.TB
}

// Given returns a fixture wired to the shared ServicesFixture
func Given(t testing.TB) BHSServiceFixture {
	return &bhsServiceFixture{
		ServicesFixture: testservices.GivenServices(t),
		t:               t,
	}
}

// NewBHSService builds a *BlockHeadersService whose resty.Client uses the
func (f *bhsServiceFixture) NewBHSService() *bhs.BlockHeadersService {
	logger := logging.NewTestLogger(f.t)
	httpClient := f.BHS().HttpClient()
	network := f.Network()

	cfg := defs.BHS{
		URL:    defs.BHSTestURL,
		APIKey: defs.BHSApiKey,
	}

	return bhs.New(httpClient, logger, network, cfg)
}
