package testabilities

import "testing"

type FixtureChaintracks interface {
	MockServer() MockServer
}

type fixtureChaintracks struct {
	testing.TB
}

func Given(t testing.TB) FixtureChaintracks {
	return &fixtureChaintracks{
		TB: t,
	}
}

func (f *fixtureChaintracks) MockServer() MockServer {
	return newMockServer(f.TB)
}
