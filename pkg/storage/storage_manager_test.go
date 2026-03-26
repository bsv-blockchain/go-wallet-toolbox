package storage_test

import (
	"fmt"
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestWalletStorageManager_GetAuth(t *testing.T) {
	t.Run("get auth successfully", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		activeStorage := given.MockProvider()

		// and
		storageManager := given.StorageManagerForUser(testusers.Alice, activeStorage)

		// and:
		mocks.SetupMockStorageProvider(t, activeStorage)

		// when
		auth, err := storageManager.GetAuth(t.Context())
		require.NoError(t, err)

		require.Equal(t, wdk.AuthID{
			UserID:      &testusers.Alice.ID,
			IdentityKey: testusers.Alice.IdentityKey(t),
			IsActive:    to.Ptr(true),
		}, auth)
	})

	errorCases := map[string]struct {
		settingsOverride func(settingsResponse *mocks.StorageProviderMethodResponse[*wdk.TableSettings])
		userOverride     func(userResponse *mocks.StorageProviderMethodResponse[*wdk.FindOrInsertUserResponse])
	}{
		"return error when active storage MakeAvailable returns an error": {
			settingsOverride: func(settingsResponse *mocks.StorageProviderMethodResponse[*wdk.TableSettings]) {
				settingsResponse.Error = fmt.Errorf("failed to make storage available")
				settingsResponse.Success = nil
			},
		},
		"return error when active storage FindOrInsertUser returns an error": {
			userOverride: func(userResponse *mocks.StorageProviderMethodResponse[*wdk.FindOrInsertUserResponse]) {
				userResponse.Error = fmt.Errorf("failed to find or insert user")
				userResponse.Success = nil
			},
		},
		"return error when user has different identity key then returned by storage": {
			userOverride: func(userResponse *mocks.StorageProviderMethodResponse[*wdk.FindOrInsertUserResponse]) {
				userResponse.Success = &wdk.FindOrInsertUserResponse{
					User: wdk.TableUser{
						UserID:        testusers.Alice.ID,
						IdentityKey:   "different-identity-key",
						ActiveStorage: "storage-id",
					},
				}
			},
		},
	}
	for name, test := range errorCases {
		t.Run(name, func(t *testing.T) {
			// given:
			given, cleanup := testabilities.Given(t)
			defer cleanup()

			// and:
			activeStorage := given.MockProvider()

			// and
			storageManager := given.StorageManagerForUser(testusers.Alice, activeStorage)

			// and:
			mocks.SetupMockStorageProvider(t, activeStorage,
				mocks.WithMakeAvailableResponse(test.settingsOverride),
				mocks.WithFindOrInsertUserResponse(test.userOverride),
			)

			// when
			auth, err := storageManager.GetAuth(t.Context())
			assert.Empty(t, auth)
			require.Error(t, err)
		})
	}

	t.Run("cache the storage answer when getting auth", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		activeStorage := given.MockProvider()

		// and
		storageManager := given.StorageManagerForUser(testusers.Alice, activeStorage)

		// and:
		mocks.SetupMockStorageProvider(t, activeStorage,
			mocks.WithMakeAvailableResponse(mocks.Once[*mocks.StorageProviderMethodResponse[*wdk.TableSettings]]()),
			mocks.WithFindOrInsertUserResponse(mocks.Once[*mocks.StorageProviderMethodResponse[*wdk.FindOrInsertUserResponse]]()),
		)

		// when:
		_, err := storageManager.GetAuth(t.Context())
		// then:
		require.NoError(t, err)

		// when: second call
		auth, err := storageManager.GetAuth(t.Context())

		// then:
		require.NoError(t, err)
		require.Equal(t, wdk.AuthID{
			UserID:      &testusers.Alice.ID,
			IdentityKey: testusers.Alice.IdentityKey(t),
			IsActive:    to.Ptr(true),
		}, auth)
	})
}

func TestWalletStorageManager_WithBackups(t *testing.T) {
	t.Run("one active one backup", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		activeStorage := given.Provider().GORMWithCleanDatabase()

		// and:
		givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
		defer cleanup()
		backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

		// and
		storageManager := given.StorageManagerForUser(testusers.Alice, activeStorage, backupProvider)

		// when:
		auth, err := storageManager.GetAuth(t.Context())

		// then:
		require.NoError(t, err)
		require.Equal(t, wdk.AuthID{
			UserID:      &testusers.Alice.ID,
			IdentityKey: testusers.Alice.IdentityKey(t),
			IsActive:    to.Ptr(true),
		}, auth)

		// and:
		require.Equal(t, fixtures.SecondStorageIdentityKey, storageManager.GetActiveStore())
	})

	t.Run("no active one backup", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
		defer cleanup()
		backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

		// and
		storageManager := given.StorageManagerForUser(testusers.Alice, nil, backupProvider)

		// when:
		auth, err := storageManager.GetAuth(t.Context())

		// then:
		require.NoError(t, err)
		require.Equal(t, wdk.AuthID{
			UserID:      &testusers.Alice.ID,
			IdentityKey: testusers.Alice.IdentityKey(t),
			IsActive:    to.Ptr(true),
		}, auth)
	})
}

func TestWalletStorageManager_SetActive(t *testing.T) {
	t.Run("one active one backup", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		activeStorage := given.Provider().GORMWithCleanDatabase()

		// and:
		givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
		defer cleanup()
		backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

		// and
		storageManager := given.StorageManagerForUser(testusers.Alice, activeStorage, backupProvider)

		// when:
		err := storageManager.SetActive(t.Context(), fixtures.StorageIdentityKey)

		// then:
		require.NoError(t, err)
		require.Equal(t, fixtures.StorageIdentityKey, storageManager.GetActiveStore())

		// when:
		const topUpAmount = 1000
		given.Faucet(activeStorage, testusers.Alice).TopUp(topUpAmount)

		// then:
		outputs, err := storageManager.ListOutputs(t.Context(), wdk.ListOutputsArgs{Limit: 1000})
		require.NoError(t, err)
		require.Equal(t, topUpAmount, int(outputs.Outputs[0].Satoshis)) //nolint:gosec // safe: satoshis fit in int for test values

		// when: switch to backup
		err = storageManager.SetActive(t.Context(), fixtures.SecondStorageIdentityKey)

		// then:
		require.NoError(t, err)
		require.Equal(t, fixtures.SecondStorageIdentityKey, storageManager.GetActiveStore())

		// and:
		outputs, err = storageManager.ListOutputs(t.Context(), wdk.ListOutputsArgs{Limit: 1000})
		require.NoError(t, err)
		require.Equal(t, topUpAmount, int(outputs.Outputs[0].Satoshis)) //nolint:gosec // safe: satoshis fit in int for test values
	})

	t.Run("one active on unexisting storage", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		activeStorage := given.Provider().GORMWithCleanDatabase()

		// and
		storageManager := given.StorageManagerForUser(testusers.Alice, activeStorage)

		// when:
		err := storageManager.SetActive(t.Context(), "unexisting-storage")

		// then:
		require.Error(t, err)
		require.Equal(t, fixtures.StorageIdentityKey, storageManager.GetActiveStore())
	})
}

func TestWalletStorageManager_FindOutputBaskets(t *testing.T) {
	t.Run("one active one backup", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		activeStorage := given.Provider().GORMWithCleanDatabase()

		// and
		storageManager := given.StorageManagerForUser(testusers.Alice, activeStorage)

		// when:
		baskets, err := storageManager.FindOutputBaskets(t.Context(), wdk.FindOutputBasketsArgs{
			Name: to.Ptr(wdk.BasketNameForChange),
		})

		// then:
		require.NoError(t, err)
		require.Len(t, baskets, 1)
		require.Equal(t, wdk.BasketNameForChange, string(baskets[0].Name))
	})
}

func TestWalletStorageManager_FindOutputs(t *testing.T) {
	t.Run("one active one backup", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		activeStorage := given.Provider().GORMWithCleanDatabase()

		// and
		storageManager := given.StorageManagerForUser(testusers.Alice, activeStorage)

		// when:
		outputs, err := storageManager.FindOutputs(t.Context(), wdk.FindOutputsArgs{
			UserID:    to.Ptr(testusers.Alice.ID),
			Spendable: to.Ptr(true),
		})

		// then:
		require.NoError(t, err)
		require.Empty(t, outputs)

		// when: top up
		const topUpAmount = 1000
		given.Faucet(activeStorage, testusers.Alice).TopUp(topUpAmount)

		// when:
		outputs, err = storageManager.FindOutputs(t.Context(), wdk.FindOutputsArgs{
			UserID:    to.Ptr(testusers.Alice.ID),
			Spendable: to.Ptr(true),
		})

		// then:
		require.NoError(t, err)
		require.Len(t, outputs, 1)
		require.Equal(t, topUpAmount, int(outputs[0].Satoshis))
	})
}
