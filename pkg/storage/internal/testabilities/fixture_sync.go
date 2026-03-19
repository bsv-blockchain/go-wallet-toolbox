package testabilities

import (
	"testing"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	pkgtestabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type SyncFixture interface {
	StorageFixture

	SeedDB(storage *storage.Provider, user testusers.User) SeedDBForSync
	RequestSyncChunk(user testusers.User) RequestSyncChunkFixture
}

type SeedDBForSync interface {
	OwnsTransaction() testvectors.TransactionSpec
	OwnsMinedTransaction() testvectors.TransactionSpec
	OwnsInternalizedAndNotProcessedTx() (internalizedTxID string, createActionResult *wdk.StorageCreateActionResult)
	PopulateTransactionsBatch(numberOfTxs int) SeedDBForSync

	SetLabels(labels ...string) SeedDBForSync

	GetAllOwnedTransactionIDs() []string
	GetAvailableBalance() uint64
}

type RequestSyncChunkFixture interface {
	NoOffsets() RequestSyncChunkFixture
	WithSince(t time.Time) RequestSyncChunkFixture
	WithMaxItems(maxItems uint64) RequestSyncChunkFixture
	WithOffset(entityName wdk.EntityName, offset uint64) RequestSyncChunkFixture

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
	testing.TB

	args wdk.RequestSyncChunkArgs
}

func (s *syncFixture) RequestSyncChunk(user testusers.User) RequestSyncChunkFixture {
	return &requestSyncChunkFixture{
		TB:   s.t,
		args: fixtures.DefaultRequestSyncChunkArgs(user.IdentityKey(s.t), s.StorageIdentityKey(), fixtures.SecondStorageIdentityKey),
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

func (s *requestSyncChunkFixture) WithOffset(entityName wdk.EntityName, offset uint64) RequestSyncChunkFixture {
	for i := range s.args.Offsets {
		if s.args.Offsets[i].Name == entityName {
			s.args.Offsets[i].Offset = offset
			return s
		}
	}
	require.Failf(s, "Offset not found", "Entity name %s not found in offsets", entityName)
	return s
}

func (s *syncFixture) SeedDB(storage *storage.Provider, user testusers.User) SeedDBForSync {
	return &seedDbForSync{
		t:              s.t,
		faucet:         s.Faucet(storage, user),
		storage:        storage,
		storageFixture: s.storageFixture,
	}
}

type seedDbForSync struct {
	t                testing.TB
	faucet           pkgtestabilities.FaucetFixture
	txCounter        int
	minedTXs         []testvectors.TransactionSpec
	notMinedTXs      []testvectors.TransactionSpec
	storageFixture   *storageFixture
	storage          *storage.Provider
	labelsForNextTxs []string
}

func (s *seedDbForSync) OwnsTransaction() testvectors.TransactionSpec {
	s.t.Helper()
	s.txCounter += 1
	txSpec, _ := s.faucet.TopUp(satoshi.MustAdd(1000, s.txCounter), pkgtestabilities.WithLabelsTopUp(s.labelsForNextTxs...))
	s.notMinedTXs = append(s.notMinedTXs, txSpec)
	return txSpec
}

func (s *seedDbForSync) OwnsMinedTransaction() testvectors.TransactionSpec {
	s.t.Helper()
	s.txCounter += 1
	opts := []pkgtestabilities.TopUpOpts{
		pkgtestabilities.WithLabelsTopUp(s.labelsForNextTxs...),
		pkgtestabilities.WithMinedTopUp(),
	}
	txSpec, _ := s.faucet.TopUp(satoshi.MustAdd(1000, s.txCounter), opts...)
	s.minedTXs = append(s.minedTXs, txSpec)
	return txSpec
}

func (s *seedDbForSync) OwnsInternalizedAndNotProcessedTx() (internalizedTxID string, createActionResult *wdk.StorageCreateActionResult) {
	s.t.Helper()
	var signedTx *transaction.Transaction
	createActionResult, signedTx = s.storageFixture.Action(s.storage).
		WithSatoshisToInternalize(99902).
		WithSatoshisToSend(1000).
		Created()

	internalizedTxID = signedTx.Inputs[0].SourceTXID.String()
	return internalizedTxID, createActionResult
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

func (s *seedDbForSync) GetAllOwnedTransactionIDs() []string {
	s.t.Helper()
	all := seq.Concat(seq.FromSlice(s.notMinedTXs), seq.FromSlice(s.minedTXs))
	return seq.Collect(
		seq.Map(all, func(spec testvectors.TransactionSpec) string {
			return spec.ID().String()
		}),
	)
}

func (s *seedDbForSync) GetAvailableBalance() uint64 {
	all := seq.Concat(seq.FromSlice(s.notMinedTXs), seq.FromSlice(s.minedTXs))
	var total uint64
	for tx := range all {
		total += tx.TX().TotalOutputSatoshis()
	}
	return total
}

func (s *seedDbForSync) SetLabels(labels ...string) SeedDBForSync {
	s.labelsForNextTxs = labels
	return s
}
