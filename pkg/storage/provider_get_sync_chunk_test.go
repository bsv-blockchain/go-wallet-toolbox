package storage_test

import (
	"math"
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
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

	thenChunk.KnownTxsCount(1)
	thenChunk.ProvenTxReqAtIndex(0).AlignsWithTxSpec(ownedTx1)

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
		KnownTxsCount(1).
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
		KnownTxsCount(0).
		ProvenTxsCount(0)
}

func TestGetSyncChunkOneByOne(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	seed := given.SeedDB(activeStorage, testusers.Alice)
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
		KnownTxsCount(0).
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
		KnownTxsCount(0)

	// given:
	args = argsFixture.WithOffset(wdk.ProvenTxEntityName, 1).Args()

	// when:
	chunk, err = activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	thenChunk = then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)
	thenChunk.BasketsCount(0).
		ProvenTxsCount(0).
		KnownTxsCount(1)
}
