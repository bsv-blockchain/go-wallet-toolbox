package testabilities

import "testing"

func New(t testing.TB) (MonitorFixture, MonitorAssertions) {
	t.Helper()
	fixture := Given(t)
	return fixture, then(t, fixture.(*monitorFixture))
}
