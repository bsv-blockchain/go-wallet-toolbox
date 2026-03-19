package storage_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	defaultBasketName   = "default"
	secondaryBasketName = "secondary"
	savingsBasketName   = "savings"

	secondaryMinUTXOValue = 50000
	savingsMinUTXOValue   = 75000

	secondaryNumberOfUTXOs = 2
	savingsNumberOfUTXOs   = 3
)

func TestOutputBasketsCRUD(t *testing.T) {
	// given:
	activeStorage := seedDBWithOutputBaskets(t, testusers.Alice)

	t.Run("find by name", func(t *testing.T) {
		// when:
		basket, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testusers.Alice.ID).
			Name().Equals(defaultBasketName).
			Find(t.Context())

		// then:
		require.NoError(t, err)
		require.Len(t, basket, 1)
		assert.Equal(t, defaultBasketName, basket[0].Name)
	})

	t.Run("filter by MinimumDesiredUTXOValue", func(t *testing.T) {
		// when:
		baskets, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testusers.Alice.ID).
			MinimumDesiredUTXOValue().Equals(secondaryMinUTXOValue).
			Find(t.Context())

		// then:
		require.NoError(t, err)
		require.Len(t, baskets, 1)
		assert.Equal(t, secondaryBasketName, baskets[0].Name)
		assert.Equal(t, uint64(secondaryMinUTXOValue), baskets[0].MinimumDesiredUTXOValue)
	})

	t.Run("count all baskets for user", func(t *testing.T) {
		// when:
		count, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testusers.Alice.ID).
			Count(t.Context())

		// then:
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})

	t.Run("update basket values", func(t *testing.T) {
		newCount := int64(5)
		newValue := uint64(100000)

		// when:
		err := activeStorage.OutputBasketsEntity().Update(t.Context(), &entity.OutputBasketUpdateSpecification{
			UserID:                  testusers.Alice.ID,
			Name:                    to.Ptr(defaultBasketName),
			NumberOfDesiredUTXOs:    &newCount,
			MinimumDesiredUTXOValue: &newValue,
		})

		// then:
		require.NoError(t, err)
		bs, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testusers.Alice.ID).
			Name().Equals(defaultBasketName).
			Find(t.Context())
		require.NoError(t, err)
		require.Len(t, bs, 1)
		assert.Equal(t, newCount, bs[0].NumberOfDesiredUTXOs)
		assert.Equal(t, newValue, bs[0].MinimumDesiredUTXOValue)
	})

	t.Run("paged listing", func(t *testing.T) {
		// when:
		list, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testusers.Alice.ID).
			Paged(1, 1, false).
			Find(t.Context())

		// then:
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, secondaryBasketName, list[0].Name)
	})

	t.Run("filter by NumberOfDesiredUTXOs", func(t *testing.T) {
		// when:
		baskets, err := activeStorage.OutputBasketsEntity().Read().
			UserID().Equals(testusers.Alice.ID).
			NumberOfDesiredUTXOs().Equals(savingsNumberOfUTXOs).
			Find(t.Context())

		// then:
		require.NoError(t, err)
		require.Len(t, baskets, 1)
		assert.Equal(t, savingsBasketName, baskets[0].Name)
		assert.Equal(t, int64(savingsNumberOfUTXOs), baskets[0].NumberOfDesiredUTXOs)
	})
}

func TestOutputBasketExposedGetter(t *testing.T) {
	activeStorage := seedDBWithOutputBaskets(t, testusers.Alice)

	t.Run("find by name", func(t *testing.T) {
		// when:
		baskets, err := activeStorage.FindOutputBasketsAuth(t.Context(), testusers.Alice.AuthID(), wdk.FindOutputBasketsArgs{
			Name: to.Ptr(defaultBasketName),
		})

		// then:
		require.NoError(t, err)
		require.Len(t, baskets, 1)
		assert.Equal(t, defaultBasketName, string(baskets[0].Name))
	})

	t.Run("filter by MinimumDesiredUTXOValue", func(t *testing.T) {
		// when:
		baskets, err := activeStorage.FindOutputBasketsAuth(t.Context(), testusers.Alice.AuthID(), wdk.FindOutputBasketsArgs{
			MinimumDesiredUTXOValue: to.Ptr(uint64(secondaryMinUTXOValue)),
		})

		// then:
		require.NoError(t, err)
		require.Len(t, baskets, 1)
		assert.Equal(t, secondaryBasketName, string(baskets[0].Name))
		assert.Equal(t, uint64(secondaryMinUTXOValue), baskets[0].MinimumDesiredUTXOValue)
	})

	t.Run("filter by NumberOfDesiredUTXOs", func(t *testing.T) {
		// when:
		baskets, err := activeStorage.FindOutputBasketsAuth(t.Context(), testusers.Alice.AuthID(), wdk.FindOutputBasketsArgs{
			NumberOfDesiredUTXOs: to.Ptr(int64(savingsNumberOfUTXOs)),
		})

		// then:
		require.NoError(t, err)
		require.Len(t, baskets, 1)
		assert.Equal(t, savingsBasketName, string(baskets[0].Name))
		assert.Equal(t, int64(savingsNumberOfUTXOs), baskets[0].NumberOfDesiredUTXOs)
	})

	t.Run("filter by UserID", func(t *testing.T) {
		// when:
		baskets, err := activeStorage.FindOutputBasketsAuth(t.Context(), testusers.Alice.AuthID(), wdk.FindOutputBasketsArgs{
			UserID: to.Ptr(testusers.Alice.ID),
		})

		// then:
		require.NoError(t, err)
		require.Len(t, baskets, 3)
	})

	t.Run("attempt to filter by another UserID", func(t *testing.T) {
		// when:
		_, err := activeStorage.FindOutputBasketsAuth(t.Context(), testusers.Alice.AuthID(), wdk.FindOutputBasketsArgs{
			UserID: to.Ptr(testusers.Bob.ID),
		})

		// then:
		require.Error(t, err)
	})
}

func seedDBWithOutputBaskets(t testing.TB, user testusers.User) *storage.Provider {
	t.Helper()

	given, cleanup := testabilities.Given(t)
	t.Cleanup(cleanup)
	activeStorage := given.Provider().GORM()

	defaultBasket := &entity.OutputBasket{
		UserID:                  user.ID,
		Name:                    secondaryBasketName,
		NumberOfDesiredUTXOs:    int64(secondaryNumberOfUTXOs),
		MinimumDesiredUTXOValue: secondaryMinUTXOValue,
	}
	require.NoError(t, activeStorage.OutputBasketsEntity().Create(t.Context(), defaultBasket))

	savingsBasket := &entity.OutputBasket{
		UserID:                  user.ID,
		Name:                    savingsBasketName,
		NumberOfDesiredUTXOs:    int64(savingsNumberOfUTXOs),
		MinimumDesiredUTXOValue: savingsMinUTXOValue,
	}
	require.NoError(t, activeStorage.OutputBasketsEntity().Create(t.Context(), savingsBasket))

	return activeStorage
}
