package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services/internal/arc"
	"github.com/go-softwarelab/common/pkg/to"
)

type ARCFServiceFixture interface {
	testabilities.ServicesFixture

	NewArcService(opts ...func(*arc.Config)) *arc.Service
}

func Given(t testing.TB) ARCFServiceFixture {
	return &arcServiceFixture{
		ServicesFixture: testabilities.Given(t),
		t:               t,
	}
}

type arcServiceFixture struct {
	testabilities.ServicesFixture
	t testing.TB
}

func (f *arcServiceFixture) NewArcService(opts ...func(*arc.Config)) *arc.Service {
	logger := logging.NewTestLogger(f.t)
	httpClient := f.ARC().HttpClient()
	network := f.Network()
	config := to.OptionsWithDefault(arc.Config{
		URL:           to.IfThen(network == defs.NetworkMainnet, defs.ArcURL).ElseThen(defs.ArcTestURL),
		Token:         to.IfThen(network == defs.NetworkMainnet, defs.ArcToken).ElseThen(defs.ArcTestToken),
		DeploymentID:  testabilities.DeploymentID,
		WaitFor:       "",
		CallbackURL:   "",
		CallbackToken: "",
	}, opts...)

	return arc.NewARCService(logger, httpClient, config)
}
