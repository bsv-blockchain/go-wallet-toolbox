package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestProvider_SetActive(t *testing.T) {
	t.Run("sets active storage for existing user", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()
		fakeStorageIdentityKey := "a"

		// when:
		err := activeStorage.SetActive(t.Context(), testusers.Alice.AuthID(), fakeStorageIdentityKey)

		// then:
		require.NoError(t, err)

		// when:
		user, err := activeStorage.FindOrInsertUser(t.Context(), testusers.Alice.IdentityKey(t))

		// then:
		require.NoError(t, err)
		require.Equal(t, fakeStorageIdentityKey, user.User.ActiveStorage)
	})

	t.Run("attempt to set active with wrong authID", func(t *testing.T) {
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// given:
		activeStorage := given.Provider().GORM()
		fakeStorageIdentityKey := "a"

		// when:
		err := activeStorage.SetActive(t.Context(), wdk.AuthID{}, fakeStorageIdentityKey)

		// then:
		require.Error(t, err)
	})
}
