package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/mocks"
)

type mockedStorageAssertion struct {
	testing.TB

	storage *mocks.MockWalletStorageProvider
}

func (w *mockedStorageAssertion) HadNoInteraction() {
	mocks.SetupMockStorageProvider(w, w.storage, mocks.ExpectNoInteraction())
}
