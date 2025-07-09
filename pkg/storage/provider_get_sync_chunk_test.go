package storage_test

import (
	"math"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	commonLabel    = "common_label"
	customLabelTx1 = "customLabelTx1"
	customLabelTx2 = "customLabelTx2"
)

func TestGetSyncChunk(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	seed := given.SeedDB(activeStorage, testusers.Alice)
	ownedTx1 := seed.SetLabels(commonLabel, customLabelTx1).OwnsTransaction()
	ownedTx2 := seed.SetLabels(commonLabel, customLabelTx2).OwnsMinedTransaction()
	internalizedTxID, createActionResult := seed.OwnsInternalizedAndNotProcessedTx()

	// and:
	args := given.RequestSyncChunk(testusers.Alice).Args()

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk := then.Chunk(chunk).WithoutError(err)

	// and:
	thenChunk.WithGeneralInfo(&args)

	// and baskets:
	thenChunk.BasketsCount(1).
		BasketAtIndex(0).WithUserID(testusers.Alice.ID).HasValidID().IsDefaultBasket()

	// and proven tx requests:
	thenChunk.ProvenTxReqsCount(2)
	thenChunk.ProvenTxReqAtIndex(0).WithTxID(internalizedTxID)
	thenChunk.ProvenTxReqAtIndex(1).AlignsWithTxSpec(ownedTx1)

	// and proven txs:
	thenChunk.ProvenTxsCount(1)
	thenChunk.ProvenTxAtIndex(0).AlignsWithTxSpec(ownedTx2).HasMerklePath()

	// and user's transactions:
	thenChunk.TransactionsCount(4)
	thenChunk.TransactionAtIndex(0).
		WithoutTxID().
		WithoutProvenTxID().
		WithReference(createActionResult.Reference)

	thenChunk.TransactionAtIndex(1).
		WithTxID(internalizedTxID).
		WithoutProvenTxID()

	thenChunk.TransactionAtIndex(2).
		WithTxID(ownedTx2.ID()).
		WithProvenTxID(chunk.ProvenTxs[0].ProvenTxID)

	thenChunk.TransactionAtIndex(3).
		WithTxID(ownedTx1.ID()).
		WithoutProvenTxID()

	// and outputs:
	thenChunk.OutputsCount(33)
	thenChunk.OutputAtIndex(0).
		WithTransactionID(chunk.Transactions[0].TransactionID).
		WithoutBasketID()

	for i := 1; i <= 29; i++ {
		thenChunk.OutputAtIndex(i).
			WithTransactionID(chunk.Transactions[0].TransactionID).
			WithBasketID(chunk.OutputBaskets[0].BasketID)
	}

	thenChunk.OutputAtIndex(30).
		WithTransactionID(chunk.Transactions[1].TransactionID).
		WithBasketID(chunk.OutputBaskets[0].BasketID)

	thenChunk.OutputAtIndex(31).
		WithTransactionID(chunk.Transactions[2].TransactionID).
		WithBasketID(chunk.OutputBaskets[0].BasketID)

	thenChunk.OutputAtIndex(32).
		WithTransactionID(chunk.Transactions[3].TransactionID).
		WithBasketID(chunk.OutputBaskets[0].BasketID)

	// and labels:
	thenChunk.
		LabelsCount(4). // 3 + 1 (from OwnsInternalizedAndNotProcessedTx)
		WithTxLabels(chunk.Transactions[2].TransactionID, commonLabel, customLabelTx2).
		WithTxLabels(chunk.Transactions[3].TransactionID, commonLabel, customLabelTx1)

	// TODO: Remember to add more assertions for other entities when implemented
}

func TestGetSyncChunkNoOffsets(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	args := given.RequestSyncChunk(testusers.Alice).
		NoOffsets().
		Args()

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	then.Chunk(chunk).WithoutError(err).
		WithGeneralInfo(&args).
		AllCountZero()
}

func TestGetSyncChunkOffsetsOverMaxItems(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	args := fixtures.DefaultRequestSyncChunkArgs(testusers.Alice.IdentityKey(t), givenProvider.StorageIdentityKey(), fixtures.SecondStorageIdentityKey)
	for i := range args.Offsets {
		args.Offsets[i].Offset = 100 // This is more than we have in the database
	}

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	then.Chunk(chunk).WithoutError(err).
		WithGeneralInfo(&args).
		AllCountZero()
}

func TestGetSyncChunkSinceAsCurrent(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	args := given.RequestSyncChunk(testusers.Alice).
		WithSince(time.Now().Add(time.Hour)). // assumes that no items are older than now+1Hour
		Args()

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	then.Chunk(chunk).WithoutError(err).
		WithFromStorageIdentityKey(args.FromStorageIdentityKey).
		WithToStorageIdentityKey(args.ToStorageIdentityKey).
		WithUserIdentityKey(args.IdentityKey).
		AllCountZero()
}

func TestGetSyncChunkSinceAsPast(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	seed := given.SeedDB(activeStorage, testusers.Alice)
	seed.SetLabels(commonLabel)
	seed.OwnsTransaction()
	seed.OwnsMinedTransaction()

	args := given.RequestSyncChunk(testusers.Alice).
		WithSince(time.Now().Add(-time.Hour)).
		Args()

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	then.Chunk(chunk).WithoutError(err).
		WithGeneralInfo(&args).
		BasketsCount(1).
		ProvenTxReqsCount(1).
		ProvenTxsCount(1).
		TransactionsCount(2).
		OutputsCount(2).
		LabelsCount(1).
		LabelsMapCount(2)
}

func TestGetSyncChunkMaxItems(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	args := given.RequestSyncChunk(testusers.Alice).
		WithMaxItems(math.MaxUint64).
		Args()

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	then.Chunk(chunk).WithoutError(err).
		WithGeneralInfo(&args).
		BasketsCount(1).
		ProvenTxReqsCount(0).
		ProvenTxsCount(0).
		TransactionsCount(0).
		OutputsCount(0).
		LabelsCount(0).
		LabelsMapCount(0)
}

func TestGetSyncChunkOneByOne(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	seed := given.SeedDB(activeStorage, testusers.Alice)
	seed.SetLabels(commonLabel)
	seed.OwnsTransaction()
	seed.OwnsMinedTransaction()

	// and:
	argsFixture := given.RequestSyncChunk(testusers.Alice).
		WithMaxItems(1)

	args := argsFixture.Args()

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk := then.Chunk(chunk).WithoutError(err)

	// and:
	thenChunk.WithGeneralInfo(&args)

	thenChunk.BasketsCount(1).
		ProvenTxReqsCount(0).
		ProvenTxsCount(0)

	// given::
	args = argsFixture.WithOffset(wdk.OutputBasketEntityName, 1).Args()

	// when:
	chunk, err = activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk = then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)
	thenChunk.BasketsCount(0).
		ProvenTxsCount(1).
		ProvenTxReqsCount(0)

	// given:
	args = argsFixture.WithOffset(wdk.ProvenTxEntityName, 1).Args()

	// when:
	chunk, err = activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk = then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)
	thenChunk.BasketsCount(0).
		ProvenTxsCount(0).
		ProvenTxReqsCount(1)

	// given:
	args = argsFixture.WithOffset(wdk.ProvenTxEntityName, 2).Args()

	// when:
	chunk, err = activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk = then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)
	thenChunk.BasketsCount(0).
		ProvenTxsCount(0).
		ProvenTxReqsCount(0).
		TransactionsCount(1)

	// given:
	args = argsFixture.WithOffset(wdk.TransactionEntityName, 1).Args()

	// when:
	chunk, err = activeStorage.GetSyncChunk(t.Context(), args)
	thenChunk = then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)
	thenChunk.BasketsCount(0).
		ProvenTxsCount(0).
		ProvenTxReqsCount(0).
		TransactionsCount(1)

	// given:
	args = argsFixture.WithOffset(wdk.TransactionEntityName, 2).Args()

	// when:
	chunk, err = activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk = then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)

	thenChunk.BasketsCount(0).
		ProvenTxsCount(0).
		ProvenTxReqsCount(0).
		TransactionsCount(0).
		OutputsCount(1)

	// given:
	args = argsFixture.WithOffset(wdk.OutputEntityName, 1).Args()

	// when:
	chunk, err = activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk = then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)

	thenChunk.BasketsCount(0).
		ProvenTxsCount(0).
		ProvenTxReqsCount(0).
		TransactionsCount(0).
		OutputsCount(1)

	// given:
	args = argsFixture.WithOffset(wdk.OutputEntityName, 2).Args()

	// when:
	chunk, err = activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk = then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)

	thenChunk.BasketsCount(0).
		ProvenTxsCount(0).
		ProvenTxReqsCount(0).
		TransactionsCount(0).
		OutputsCount(0).
		LabelsCount(1)

	// given:
	args = argsFixture.WithOffset(wdk.TxLabelEntityName, 1).Args()

	// when:
	chunk, err = activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk = then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)

	thenChunk.BasketsCount(0).
		ProvenTxsCount(0).
		ProvenTxReqsCount(0).
		TransactionsCount(0).
		OutputsCount(0).
		LabelsCount(0).
		LabelsMapCount(1)
}
