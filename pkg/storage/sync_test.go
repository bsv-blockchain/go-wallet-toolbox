package storage_test

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
)

func TestSyncProcess(t *testing.T) {
	//testmode.DevelopmentOnly_SetFileSQLiteMode(t)
	givenSourceDB, cleanup := testabilities.Given(t)
	defer cleanup()

	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM() // this automatically creates test-users and their default baskets

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	storageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	inserts, updates, err := storageManager.SyncToWriter(t.Context(), testusers.Alice.AuthID(), backupProvider)

	require.NoError(t, err)
	require.Equal(t, 4, inserts)
	require.Equal(t, 0, updates)
}
