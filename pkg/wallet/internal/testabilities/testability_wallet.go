package testabilities

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/mocks"
)

func New(tb testing.TB) (given WalletFixture, then WalletAssertions, cleanup func()) {
	g, cleanup := newGiven(tb)
	t := newThen(g, tb)
	return g, t, cleanup
}

type WalletAssertions interface {
	Result(any) AnyWalletResultAssertion
	Storage() MockedStorageAssertion
}

type AnyWalletResultAssertion interface {
	HasError(error)
}

type MockedStorageAssertion interface {
	HadNoInteraction()
}

type walletAssertions struct {
	testing.TB

	fixture *walletFixture
}

func newThen(g *walletFixture, t testing.TB) *walletAssertions {
	return &walletAssertions{
		TB:      t,
		fixture: g,
	}
}

func (w *walletAssertions) Result(result any) AnyWalletResultAssertion {
	return &anyResultAssertion{
		TB:     w.TB,
		result: result,
	}
}

func (w *walletAssertions) Storage() MockedStorageAssertion {
	require.Lenf(w, w.fixture.usersSetups, 1, "invalid test setup: expected exactly one user wallet setup to check it's storage: %v", w.fixture.usersSetups)

	var setup *userWalletSetup
	for s := range maps.Values(w.fixture.usersSetups) {
		setup = s
		break
	}
	require.NotNil(w, setup)
	require.Equalf(w, StorageTypeMocked, setup.storageType, "invalid test setup: expected storage type to be mocked for check on storage calls")
	require.IsType(w, &mocks.MockWalletStorageProvider{}, setup.storage, "invalid test setup: expected storage to be mocked for check on storage calls")

	return &mockedStorageAssertion{
		TB:      w.TB,
		storage: setup.storage.(*mocks.MockWalletStorageProvider),
	}
}
