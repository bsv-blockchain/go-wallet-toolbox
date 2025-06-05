package storage_test

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)
import "github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"

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
	assert.Equal(t, wdk.DefaultBasketConfiguration(), defaultBasket.BasketConfiguration)
}
