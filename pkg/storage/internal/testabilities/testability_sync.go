package testabilities

import "testing"

func NewSync(t testing.TB) (SyncFixture, SyncAssertion, func()) {
	given, cleanup := GivenSyncFixture(t)
	then := ThenSync(t)

	return given, then, cleanup
}
