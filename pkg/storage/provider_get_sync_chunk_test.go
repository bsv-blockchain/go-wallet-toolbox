package storage_test

import (
	"math"
	"testing"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// General TODO: Seed database with some data for testing

func TestGetSyncChunk(t *testing.T) {
	given, cleanup := testabilities.Given(t)
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
	// TODO: Remember to add more assertions for other entities when implemented
}

func TestGetSyncChunkNoOffsets(t *testing.T) {
	given, cleanup := testabilities.Given(t)
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
	given, cleanup := testabilities.Given(t)
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
	given, cleanup := testabilities.Given(t)
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
	given, cleanup := testabilities.Given(t)
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
	given, cleanup := testabilities.Given(t)
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
