package storage_test

import (
	"math"
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/testabilities"
	storagetestabilities "github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// General TODO: Seed database with some data for testing

func TestGetSyncChunk(t *testing.T) {
	given, cleanup := storagetestabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	givenFaucet := given.Faucet(activeStorage, testusers.Alice)
	ownedTx1, _ := givenFaucet.TopUp(100_001)
	ownedMinedTx2, _ := givenFaucet.TopUp(100_002, testabilities.WithMinedTopUp())

	args := wdk.RequestSyncChunkArgs{
		FromStorageIdentityKey: "from_storage",
		ToStorageIdentityKey:   "to_storage",
		IdentityKey:            testusers.Alice.IdentityKey(t),
		MaxItems:               10,
		MaxRoughSize:           100_000,

		Offsets: []wdk.SyncOffsets{
			{
				Name:   wdk.OutputBasketEntityName,
				Offset: 0,
			},
			{
				Name:   wdk.ProvenTxReqEntityName,
				Offset: 0,
			},
			// TODO: Add more offsets for other entities when implemented
		},
	}

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	require.NoError(t, err)
	assert.Equal(t, "from_storage", chunk.FromStorageIdentityKey)
	assert.Equal(t, "to_storage", chunk.ToStorageIdentityKey)
	assert.Equal(t, args.IdentityKey, chunk.UserIdentityKey)

	assert.Equal(t, args.IdentityKey, chunk.User.IdentityKey)
	assert.Equal(t, givenProvider.StorageIdentityKey(), chunk.User.ActiveStorage)

	require.Len(t, chunk.OutputBaskets, 1)
	defaultBasket := chunk.OutputBaskets[0]
	assert.Equal(t, testusers.Alice.ID, defaultBasket.UserID)
	assert.True(t, defaultBasket.BasketID > 0)
	assert.Equal(t, wdk.DefaultBasketConfiguration(), defaultBasket.BasketConfiguration)

	require.Len(t, chunk.ProvenTxReqs, 2)
	assert.Equal(t, chunk.ProvenTxReqs[0].TxID, ownedMinedTx2.ID())
	assert.Equal(t, []byte(chunk.ProvenTxReqs[0].RawTx), ownedMinedTx2.TX().Bytes())
	assert.Equal(t, chunk.ProvenTxReqs[1].TxID, ownedTx1.ID())
	assert.Equal(t, []byte(chunk.ProvenTxReqs[1].RawTx), ownedTx1.TX().Bytes())

	require.Len(t, chunk.ProvenTxs, 1)
	assert.Equal(t, chunk.ProvenTxs[0].TxID, ownedMinedTx2.ID())
	assert.Equal(t, []byte(chunk.ProvenTxs[0].RawTx), ownedMinedTx2.TX().Bytes())
	assert.NotEmpty(t, chunk.ProvenTxs[0].MerklePath)

	// TODO: Remember to add more assertions for other entities when implemented
}

func TestGetSyncChunkNoOffsets(t *testing.T) {
	given, cleanup := storagetestabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	args := wdk.RequestSyncChunkArgs{
		FromStorageIdentityKey: "from_storage",
		ToStorageIdentityKey:   "to_storage",
		IdentityKey:            testusers.Alice.IdentityKey(t),
		MaxItems:               10,
		MaxRoughSize:           100_000,
	}

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	require.NoError(t, err)
	assert.Equal(t, "from_storage", chunk.FromStorageIdentityKey)
	assert.Equal(t, "to_storage", chunk.ToStorageIdentityKey)
	assert.Equal(t, args.IdentityKey, chunk.UserIdentityKey)

	assert.Equal(t, args.IdentityKey, chunk.User.IdentityKey)
	assert.Equal(t, givenProvider.StorageIdentityKey(), chunk.User.ActiveStorage)

	require.Len(t, chunk.OutputBaskets, 0)
	// TODO: Remember to add more assertions for other entities when implemented
}

func TestGetSyncChunkOffsetsOverMaxItems(t *testing.T) {
	given, cleanup := storagetestabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	args := wdk.RequestSyncChunkArgs{
		FromStorageIdentityKey: "from_storage",
		ToStorageIdentityKey:   "to_storage",
		IdentityKey:            testusers.Alice.IdentityKey(t),
		MaxItems:               10,
		MaxRoughSize:           100_000,

		Offsets: []wdk.SyncOffsets{
			{
				Name:   wdk.OutputBasketEntityName,
				Offset: 100, // This is more than we have in the database
			},
			// TODO: Add more offsets for other entities when implemented
		},
	}

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	require.NoError(t, err)
	assert.Equal(t, "from_storage", chunk.FromStorageIdentityKey)
	assert.Equal(t, "to_storage", chunk.ToStorageIdentityKey)
	assert.Equal(t, args.IdentityKey, chunk.UserIdentityKey)

	assert.Equal(t, args.IdentityKey, chunk.User.IdentityKey)
	assert.Equal(t, givenProvider.StorageIdentityKey(), chunk.User.ActiveStorage)

	require.Len(t, chunk.OutputBaskets, 0)
	// TODO: Remember to add more assertions for other entities when implemented
}

func TestGetSyncChunkSinceAsCurrent(t *testing.T) {
	given, cleanup := storagetestabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	args := wdk.RequestSyncChunkArgs{
		FromStorageIdentityKey: "from_storage",
		ToStorageIdentityKey:   "to_storage",
		IdentityKey:            testusers.Alice.IdentityKey(t),
		MaxItems:               10,
		MaxRoughSize:           100_000,
		Since:                  to.Ptr(time.Now()), // I assume that no items are older than now

		Offsets: []wdk.SyncOffsets{
			{
				Name:   wdk.OutputBasketEntityName,
				Offset: 0,
			},
			// TODO: Add more offsets for other entities when implemented
		},
	}

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	require.NoError(t, err)
	assert.Equal(t, "from_storage", chunk.FromStorageIdentityKey)
	assert.Equal(t, "to_storage", chunk.ToStorageIdentityKey)
	assert.Equal(t, args.IdentityKey, chunk.UserIdentityKey)

	assert.Nil(t, chunk.User)
	require.Len(t, chunk.OutputBaskets, 0)
	// TODO: Remember to add more assertions for other entities when implemented
}

func TestGetSyncChunkSinceAsPast(t *testing.T) {
	given, cleanup := storagetestabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	args := wdk.RequestSyncChunkArgs{
		FromStorageIdentityKey: "from_storage",
		ToStorageIdentityKey:   "to_storage",
		IdentityKey:            testusers.Alice.IdentityKey(t),
		MaxItems:               10,
		MaxRoughSize:           100_000,
		Since:                  to.Ptr(time.Now().Add(-time.Hour)),

		Offsets: []wdk.SyncOffsets{
			{
				Name:   wdk.OutputBasketEntityName,
				Offset: 0,
			},
			// TODO: Add more offsets for other entities when implemented
		},
	}

	// when:
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	require.NoError(t, err)
	assert.Equal(t, "from_storage", chunk.FromStorageIdentityKey)
	assert.Equal(t, "to_storage", chunk.ToStorageIdentityKey)
	assert.Equal(t, args.IdentityKey, chunk.UserIdentityKey)

	// then:
	require.NoError(t, err)
	assert.Equal(t, "from_storage", chunk.FromStorageIdentityKey)
	assert.Equal(t, "to_storage", chunk.ToStorageIdentityKey)
	assert.Equal(t, args.IdentityKey, chunk.UserIdentityKey)

	assert.Equal(t, args.IdentityKey, chunk.User.IdentityKey)
	assert.Equal(t, givenProvider.StorageIdentityKey(), chunk.User.ActiveStorage)

	require.Len(t, chunk.OutputBaskets, 1)
	// TODO: Remember to add more assertions for other entities when implemented
}

func TestGetSyncChunkMaxItems(t *testing.T) {
	given, cleanup := storagetestabilities.Given(t)
	defer cleanup()

	// given:
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	args := wdk.RequestSyncChunkArgs{
		FromStorageIdentityKey: "from_storage",
		ToStorageIdentityKey:   "to_storage",
		IdentityKey:            testusers.Alice.IdentityKey(t),
		MaxItems:               math.MaxUint64,
		MaxRoughSize:           100_000,

		Offsets: []wdk.SyncOffsets{
			{
				Name:   wdk.OutputBasketEntityName,
				Offset: 0,
			},
		},
	}

	// when:
	_, err := activeStorage.GetSyncChunk(t.Context(), args)

	// then:
	require.NoError(t, err)
}
