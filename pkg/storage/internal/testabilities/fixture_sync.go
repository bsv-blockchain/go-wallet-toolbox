package testabilities

import (
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	pkgtestabilities "github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
)

type SyncFixture interface {
	StorageFixture

	SeedDB(storage *storage.Provider, user testusers.User) SeedDBForSync
}

type SeedDBForSync interface {
	OwnsTransaction() testvectors.TransactionSpec
	OwnsMinedTransaction() testvectors.TransactionSpec
}

type syncFixture struct {
	*storageFixture
}

func GivenSyncFixture(t testing.TB) (SyncFixture, func()) {
	given, cleanup := Given(t)
	return &syncFixture{
		storageFixture: given.(*storageFixture),
	}, cleanup
}

func (s *syncFixture) SeedDB(storage *storage.Provider, user testusers.User) SeedDBForSync {
	return &seedDbForSync{
		t:      s.t,
		faucet: s.Faucet(storage, user),
	}
}

type seedDbForSync struct {
	t         testing.TB
	faucet    pkgtestabilities.FaucetFixture
	txCounter int
}

func (s *seedDbForSync) OwnsTransaction() testvectors.TransactionSpec {
	s.t.Helper()
	s.txCounter += 1
	txSpec, _ := s.faucet.TopUp(satoshi.MustAdd(1000, s.txCounter))
	return txSpec
}

func (s *seedDbForSync) OwnsMinedTransaction() testvectors.TransactionSpec {
	s.t.Helper()
	s.txCounter += 1
	txSpec, _ := s.faucet.TopUp(satoshi.MustAdd(1000, s.txCounter), pkgtestabilities.WithMinedTopUp())
	return txSpec
}
