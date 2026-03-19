package testabilities

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arc"
)

type ARCFServiceFixture interface {
	testservices.ServicesFixture

	NewArcService(opts ...func(*arc.Config)) *arc.Service
}

func Given(t testing.TB) ARCFServiceFixture {
	return &arcServiceFixture{
		ServicesFixture: testservices.GivenServices(t),
		t:               t,
	}
}

type arcServiceFixture struct {
	testservices.ServicesFixture

	t testing.TB
}

func (f *arcServiceFixture) NewArcService(opts ...func(*arc.Config)) *arc.Service {
	logger := logging.NewTestLogger(f.t)
	httpClient := f.ARC().HttpClient()
	network := f.Network()
	config := to.OptionsWithDefault(arc.Config{
		URL:           to.IfThen(network == defs.NetworkMainnet, defs.ArcURL).ElseThen(defs.ArcTestURL),
		Token:         to.IfThen(network == defs.NetworkMainnet, defs.ArcToken).ElseThen(defs.ArcTestToken),
		DeploymentID:  testservices.DeploymentID,
		WaitFor:       "",
		CallbackURL:   "",
		CallbackToken: "",
	}, opts...)

	return arc.New(logger, httpClient, config)
}
