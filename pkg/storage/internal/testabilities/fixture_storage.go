package testabilities

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type StorageFixture interface {
	testabilities.StorageFixture
	StorageManagerForUser(user testusers.User, activeStorage wdk.WalletStorageProvider, backups ...wdk.WalletStorageProvider) *storage.WalletStorageManager
	Action(activeStorage *storage.Provider) TxGeneratorFixture
}

type storageFixture struct {
	testabilities.StorageFixture

	t testing.TB
}

func (s *storageFixture) StorageManagerForUser(user testusers.User, activeStorage wdk.WalletStorageProvider, backups ...wdk.WalletStorageProvider) *storage.WalletStorageManager {
	return storage.NewWalletStorageManager(user.IdentityKey(s.t), logging.NewTestLogger(s.t), activeStorage, backups...)
}

func (s *storageFixture) Action(activeStorage *storage.Provider) TxGeneratorFixture {
	return &txGeneratorFixture{
		TB:                    s.t,
		satoshisToInternalize: fixtures.DefaultCreateActionOutputSatoshis,
		satoshisToSend:        1,
		parent:                s,
		activeStorage:         activeStorage,
		sender:                testusers.Alice,
		recipient:             testusers.Bob,
	}
}

func Given(t testing.TB) (given StorageFixture, cleanup func()) {
	storageFxt, cleanupFunc := testabilities.Given(t)

	return &storageFixture{
		t:              t,
		StorageFixture: storageFxt,
	}, cleanupFunc
}

func GivenCustomStorage(t testing.TB, identityKey, name string) (given StorageFixture, cleanup func()) {
	storageFxt, cleanupFunc := testabilities.GivenCustomStorage(t, identityKey, name)

	return &storageFixture{
		t:              t,
		StorageFixture: storageFxt,
	}, cleanupFunc
}
