package testabilities

import (
	"maps"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func New(tb testing.TB) (given WalletFixture, then WalletAssertions) {
	g := newGiven(tb)
	t := newThen(g, tb)
	return g, t
}

type WalletAssertions interface {
	Result(any) AnyWalletResultAssertion
	Storage()
}

type AnyWalletResultAssertion interface {
	HasError(error)
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

func (w *walletAssertions) Storage() {
	require.Lenf(w, w.fixture.usersSetups, 1, "invalid test setup: expected exactly one user wallet setup to check it's storage: %v", w.fixture.usersSetups)

	var setup *userWalletSetup
	for s := range maps.Values(w.fixture.usersSetups) {
		setup = s
		break
	}
	require.NotNil(w, setup)
	require.Equalf(w, setup.storageType, StorageTypeMocked, "invalid test setup: expected storage type to be mocked for check on storage calls")
	require.IsType(w, &mocks.MockWalletStorageProvider{}, setup.storage, "invalid test setup: expected storage to be mocked for check on storage calls")
	mockedStorage := setup.storage.(*mocks.MockWalletStorageProvider)
	mockedStorage.EXPECT().InternalizeAction(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(0)
}

type anyResultAssertion struct {
	testing.TB
	result any
}

func (a *anyResultAssertion) HasError(err error) {
	assert.Nil(a, a.result, "Expect nil result when receiving error")
	require.Error(a, err)
}
