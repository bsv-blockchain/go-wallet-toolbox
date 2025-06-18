package testabilities

import (
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	pkgtestabilities "github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
)

type SyncFixture interface {
	StorageFixture

	SeedDB(storage *storage.Provider, user testusers.User) SeedDBForSync
	RequestSyncChunk(user testusers.User) RequestSyncChunkFixture
}

type SeedDBForSync interface {
	OwnsTransaction() testvectors.TransactionSpec
	OwnsMinedTransaction() testvectors.TransactionSpec
	PopulateTransactionsBatch(numberOfTxs int) SeedDBForSync
}

type RequestSyncChunkFixture interface {
	NoOffsets() RequestSyncChunkFixture
	WithSince(t time.Time) RequestSyncChunkFixture
	WithMaxItems(maxItems uint64) RequestSyncChunkFixture
	Args() wdk.RequestSyncChunkArgs
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

type requestSyncChunkFixture struct {
	args wdk.RequestSyncChunkArgs
}

func (s *syncFixture) RequestSyncChunk(user testusers.User) RequestSyncChunkFixture {
	return &requestSyncChunkFixture{
		args: fixtures.DefaultRequestSyncChunkArgs(user.IdentityKey(s.t), s.StorageIdentityKey()),
	}
}

func (s *requestSyncChunkFixture) Args() wdk.RequestSyncChunkArgs {
	return s.args
}

func (s *requestSyncChunkFixture) NoOffsets() RequestSyncChunkFixture {
	s.args.Offsets = nil
	return s
}

func (s *requestSyncChunkFixture) WithSince(t time.Time) RequestSyncChunkFixture {
	s.args.Since = to.Ptr(t)
	return s
}

func (s *requestSyncChunkFixture) WithMaxItems(maxItems uint64) RequestSyncChunkFixture {
	s.args.MaxItems = maxItems
	return s
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

func (s *seedDbForSync) PopulateTransactionsBatch(numberOfTxs int) SeedDBForSync {
	for i := 0; i < numberOfTxs; i++ {
		if i%2 == 0 {
			s.OwnsMinedTransaction()
		} else {
			s.OwnsTransaction()
		}
	}

	return s
}
