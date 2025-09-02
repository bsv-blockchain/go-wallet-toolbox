package nosendtest

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func New(t testing.TB, user testusers.User) (NoSendFixture, NoSendAct, NosendAssertion, func()) {
	t.Helper()
	givenStorage, cleanup := testabilities.Given(t)
	activeProvider := givenStorage.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	given := &noSendFixture{
		TB:             t,
		StorageFixture: givenStorage,
		user:           user,
		activeProvider: activeProvider,
	}

	when := &noSendAct{
		TB:                      t,
		user:                    user,
		activeProvider:          activeProvider,
		satsToSend:              1,
		allRemainedNoSendChange: make(map[wdk.OutPoint]struct{}),
	}

	then := &nosendAssertion{
		TB:  t,
		act: when,
	}

	return given, when, then, cleanup
}
