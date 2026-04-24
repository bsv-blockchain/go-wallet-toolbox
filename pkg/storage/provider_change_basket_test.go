package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestWithChangeBasket_SetsDefaultsForNewUsers(t *testing.T) {
	// given: provider initialized with custom change basket config
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().
		WithChangeBasket(defs.ChangeBasket{
			NumberOfDesiredUTXOs:    5000,
			MinimumDesiredUTXOValue: 2000,
			MaxChangeOutputsPerTx:   20,
		}).
		GORMWithCleanDatabase()

	// when: new user is created
	resp, err := activeStorage.FindOrInsertUser(t.Context(), testusers.Alice.IdentityKey(t))

	// then:
	require.NoError(t, err)
	assert.True(t, resp.IsNew)

	basket, err := activeStorage.OutputBasketsEntity().Read().
		UserID().Equals(resp.User.UserID).
		Name().Equals(wdk.BasketNameForChange).
		Find(t.Context())

	require.NoError(t, err)
	require.Len(t, basket, 1)
	assert.Equal(t, int64(5000), basket[0].NumberOfDesiredUTXOs)
	assert.Equal(t, uint64(2000), basket[0].MinimumDesiredUTXOValue)
}

func TestSetDefaultChangeBasket_AffectsOnlySubsequentNewUsers(t *testing.T) {
	// given: provider with default config
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORMWithCleanDatabase()

	// and: alice is created with the original defaults
	respAlice, err := activeStorage.FindOrInsertUser(t.Context(), testusers.Alice.IdentityKey(t))
	require.NoError(t, err)

	// when: default change basket is updated at runtime
	activeStorage.SetDefaultChangeBasket(wdk.BasketConfiguration{
		Name:                    wdk.BasketNameForChange,
		NumberOfDesiredUTXOs:    10000,
		MinimumDesiredUTXOValue: 500,
	})

	// and: bob is created after the update
	respBob, err := activeStorage.FindOrInsertUser(t.Context(), testusers.Bob.IdentityKey(t))
	require.NoError(t, err)

	// then: alice keeps original defaults
	aliceBaskets, err := activeStorage.OutputBasketsEntity().Read().
		UserID().Equals(respAlice.User.UserID).
		Name().Equals(wdk.BasketNameForChange).
		Find(t.Context())
	require.NoError(t, err)
	require.Len(t, aliceBaskets, 1)
	assert.Equal(t, int64(32), aliceBaskets[0].NumberOfDesiredUTXOs)

	// and: bob gets the new defaults
	bobBaskets, err := activeStorage.OutputBasketsEntity().Read().
		UserID().Equals(respBob.User.UserID).
		Name().Equals(wdk.BasketNameForChange).
		Find(t.Context())
	require.NoError(t, err)
	require.Len(t, bobBaskets, 1)
	assert.Equal(t, int64(10000), bobBaskets[0].NumberOfDesiredUTXOs)
	assert.Equal(t, uint64(500), bobBaskets[0].MinimumDesiredUTXOValue)
}

func TestUpdateChangeBasket_UpdatesSpecificUserInDB(t *testing.T) {
	// given: provider with two existing users
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORMWithCleanDatabase()

	respAlice, err := activeStorage.FindOrInsertUser(t.Context(), testusers.Alice.IdentityKey(t))
	require.NoError(t, err)
	respBob, err := activeStorage.FindOrInsertUser(t.Context(), testusers.Bob.IdentityKey(t))
	require.NoError(t, err)

	// when: only alice's basket is updated
	err = activeStorage.UpdateChangeBasket(t.Context(), testusers.Alice.IdentityKey(t), wdk.BasketConfiguration{
		NumberOfDesiredUTXOs:    9999,
		MinimumDesiredUTXOValue: 777,
	})
	require.NoError(t, err)

	// then: alice's basket reflects the update
	aliceBaskets, err := activeStorage.OutputBasketsEntity().Read().
		UserID().Equals(respAlice.User.UserID).
		Name().Equals(wdk.BasketNameForChange).
		Find(t.Context())
	require.NoError(t, err)
	require.Len(t, aliceBaskets, 1)
	assert.Equal(t, int64(9999), aliceBaskets[0].NumberOfDesiredUTXOs)
	assert.Equal(t, uint64(777), aliceBaskets[0].MinimumDesiredUTXOValue)

	// and: bob's basket is unchanged
	bobBaskets, err := activeStorage.OutputBasketsEntity().Read().
		UserID().Equals(respBob.User.UserID).
		Name().Equals(wdk.BasketNameForChange).
		Find(t.Context())
	require.NoError(t, err)
	require.Len(t, bobBaskets, 1)
	assert.Equal(t, int64(32), bobBaskets[0].NumberOfDesiredUTXOs)
}

func TestUpdateChangeBasket_ReturnsErrorForUnknownUser(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	activeStorage := given.Provider().GORMWithCleanDatabase()

	// when: updating a user that doesn't exist
	err := activeStorage.UpdateChangeBasket(t.Context(), "nonexistent-key", wdk.BasketConfiguration{
		NumberOfDesiredUTXOs: 100,
	})

	// then:
	require.Error(t, err)
}
