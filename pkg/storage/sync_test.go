package storage_test

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
)

func TestSyncProcess(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.Given(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()

	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	// when:
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), testusers.Alice.AuthID(), backupProvider)

	// then:
	require.NoError(t, err)
	require.Equal(t, 0, inserts) // TODO: Adjust it after implementing sync logic
	require.Equal(t, 0, updates) // TODO: Adjust it after implementing sync logic
}

func TestSyncProcessWithMergeUser(t *testing.T) {
	// given:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORM()

	// and:
	givenSourceDB, cleanup := testabilities.Given(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()

	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	// when:
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), testusers.Alice.AuthID(), backupProvider)

	// then:
	require.NoError(t, err)
	require.Equal(t, 0, inserts) // TODO: Adjust it after implementing sync logic
	require.Equal(t, 1, updates) // TODO: Adjust it after implementing sync logic
}

func TestSyncProcessInvalidUser(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.Given(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()

	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	// when:
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), testusers.Bob.AuthID(), backupProvider)

	// then:
	require.NoError(t, err)
	require.Equal(t, 0, inserts) // TODO: Adjust it after implementing sync logic
	require.Equal(t, 0, updates) // TODO: Adjust it after implementing sync logic
}
