package storage_test

import (
	"math"
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
)

func TestGetSyncChunk(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	seed := given.SeedDB(activeStorage, testusers.Alice)
	ownedTx1 := seed.OwnsTransaction()
	ownedTx2 := seed.OwnsMinedTransaction()

	// and:
	args := given.RequestSyncChunk(testusers.Alice).Args()

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk := then.Chunk(chunk).WithoutError(err)

	// and:
	thenChunk.WithGeneralInfo(&args)

	thenChunk.BasketsCount(1).
		BasketAtIndex(0).WithUserID(testusers.Alice.ID).HasValidID().IsDefaultBasket()

	thenChunk.ProvenTxReqsCount(2)
	thenChunk.ProvenTxReqAtIndex(0).AlignsWithTxSpec(ownedTx2).HasProvenTxID()
	thenChunk.ProvenTxReqAtIndex(1).AlignsWithTxSpec(ownedTx1)

	thenChunk.ProvenTxsCount(1)
	thenChunk.ProvenTxAtIndex(0).AlignsWithTxSpec(ownedTx2).HasMerklePath()

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

	args := fixtures.DefaultRequestSyncChunkArgs(testusers.Alice.IdentityKey(t), givenProvider.StorageIdentityKey())
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
		WithSince(time.Now()). // assumes that no items are older than now
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
		ProvenTxReqsCount(2).
		ProvenTxsCount(1)
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
		ProvenTxsCount(0)
}
