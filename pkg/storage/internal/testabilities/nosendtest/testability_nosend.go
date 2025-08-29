package nosendtest

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func New(t testing.TB, user testusers.User, storageFixture testabilities.StorageFixture, activeProvider *storage.Provider) (NosendFixture, NosendAct, NosendAssertion) {
	t.Helper()
	given := &nosendFixture{
		TB:             t,
		storageFixture: storageFixture,
		user:           user,
		activeProvider: activeProvider,
	}

	when := &nosendAct{
		TB:                      t,
		user:                    user,
		activeProvider:          activeProvider,
		satsToSend:              1,
		allRemainedNoSendChange: make(map[wdk.OutPoint]struct{}),
	}

	then := &nosendAssertion{
		TB:   t,
		act: when,
	}

	return given, when, then
}
