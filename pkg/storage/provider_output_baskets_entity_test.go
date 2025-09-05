package storage_test

import (
	"testing"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputBasketsCRUD(t *testing.T) {
	// given:
	activeStorage := seedDBWithOutputBaskets(t)

	t.Run("find by name", func(t *testing.T) {
		// when:
		basket, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testUser.ID).
			Name().Equals("default").
			Find(t.Context())

		// then:
		require.NoError(t, err)
		require.Len(t, basket, 1)
		assert.Equal(t, "default", basket[0].Name)
	})

	t.Run("filter by MinimumDesiredUTXOValue", func(t *testing.T) {
		// when:
		baskets, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testUser.ID).
			MinimumDesiredUTXOValue().Equals(50000).
			Find(t.Context())

		// then:
		require.NoError(t, err)
		require.Len(t, baskets, 1)
		assert.Equal(t, "default", baskets[0].Name)
		assert.Equal(t, uint64(50000), baskets[0].MinimumDesiredUTXOValue)
	})

	t.Run("count all baskets for user", func(t *testing.T) {
		// when:
		count, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testUser.ID).
			Count(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("update basket values", func(t *testing.T) {
		newCount := int64(5)
		newValue := uint64(100000)

		// when:
		err := activeStorage.OutputBasketsEntity().Update(t.Context(), &entity.OutputBasketUpdateSpecification{
			UserID:                  testUser.ID,
			Name:                    to.Ptr("default"),
			NumberOfDesiredUTXOs:    &newCount,
			MinimumDesiredUTXOValue: &newValue,
		})

		// then:
		require.NoError(t, err)
		bs, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testUser.ID).
			Name().Equals("default").
			Find(t.Context())
		require.NoError(t, err)
		require.Len(t, bs, 1)
		assert.Equal(t, newCount, bs[0].NumberOfDesiredUTXOs)
		assert.Equal(t, newValue, bs[0].MinimumDesiredUTXOValue)
	})

	t.Run("paged listing", func(t *testing.T) {
		// when:
		list, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testUser.ID).
			Paged(1, 1, false).
			Find(t.Context())

		// then:
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "savings", list[0].Name)
	})

	t.Run("filter by NumberOfDesiredUTXOs", func(t *testing.T) {
		// when:
		baskets, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testUser.ID).
			NumberOfDesiredUTXOs().Equals(3).
			Find(t.Context())

		// then:
		require.NoError(t, err)
		require.Len(t, baskets, 1)
		assert.Equal(t, "savings", baskets[0].Name)
		assert.Equal(t, int64(3), baskets[0].NumberOfDesiredUTXOs)
	})

}

func seedDBWithOutputBaskets(t testing.TB) *storage.Provider {
	// given:
	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	activeStorage := given.Provider().GORM()

	userID := testUser.ID

	defaultBasket := &entity.OutputBasket{
		UserID:                  userID,
		Name:                    "default",
		NumberOfDesiredUTXOs:    int64(2),
		MinimumDesiredUTXOValue: 50000,
	}
	require.NoError(t, activeStorage.OutputBasketsEntity().Create(t.Context(), defaultBasket))

	savingsBasket := &entity.OutputBasket{
		UserID:                  userID,
		Name:                    "savings",
		NumberOfDesiredUTXOs:    int64(3),
		MinimumDesiredUTXOValue: 75000,
	}
	require.NoError(t, activeStorage.OutputBasketsEntity().Create(t.Context(), savingsBasket))

	return activeStorage
}

var testUser = struct{ ID int }{ID: 123}
