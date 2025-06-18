package storage_test

import (
	"fmt"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncProcess(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.GivenSyncFixture(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()
	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	seed := givenSourceDB.SeedDB(sourceProvider, testusers.Alice)
	seed.OwnsMinedTransaction()
	seed.OwnsTransaction()

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	// when:
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 2, inserts)
	assert.Equal(t, 1, updates)

	// TODO: Check if the data is actually synced. FindProvenTxReqs can be used (it is in a public interface, so we need to implement it anyway)
}

func TestSyncProcessOnlyUsers(t *testing.T) {
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
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 0, inserts)
	assert.Equal(t, 1, updates)
}

func TestSyncWithManyCustomBaskets(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.Given(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()

	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	// and:
	const customBasketsCount = 20
	for i := 0; i < customBasketsCount; i++ {
		err := sourceProvider.ConfigureBasket(t.Context(), testusers.Alice.AuthID(), wdk.BasketConfiguration{
			Name:                    primitives.StringUnder300(fmt.Sprintf("Custom_Basket_%d", i)),
			NumberOfDesiredUTXOs:    int64(i),
			MinimumDesiredUTXOValue: uint64(i),
		})
		require.NoError(t, err)
	}

	// when:
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, customBasketsCount, inserts)
	assert.Equal(t, 1, updates)
}

func TestSyncProcessWithManyTransactions(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.GivenSyncFixture(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()
	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	seed := givenSourceDB.SeedDB(sourceProvider, testusers.Alice)

	const maxItemsPerSingleSync = 10
	numberOfTxs := int(maxItemsPerSingleSync * 2.5)
	seed.PopulateTransactionsBatch(numberOfTxs)

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	// when:
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider, wdk.WithMaxSyncItems(maxItemsPerSingleSync))

	// then:
	require.NoError(t, err)
	assert.Equal(t, numberOfTxs, inserts)
	assert.Equal(t, 1, updates)

	// TODO: Check if the data is actually synced. FindProvenTxReqs can be used (it is in a public interface, so we need to implement it anyway)
}

func TestSyncProcessWithMergeUser(t *testing.T) {
	// given:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	//NOTE: Backup storage is created first, so the user data will be older than in the source storage - so the merge will happen
	backupProvider := givenBackupDB.Provider().GORM()

	// and:
	givenSourceDB, cleanup := testabilities.Given(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()

	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	// when:
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 0, inserts)
	assert.Equal(t, 2, updates)
}

func TestSyncWhereOtherUserAlreadyExist(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.Given(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()

	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()
	_, err := backupProvider.FindOrInsertUser(t.Context(), testusers.Bob.IdentityKey(t))
	require.NoError(t, err)

	// when:
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 0, inserts)
	assert.Equal(t, 1, updates)
}

func TestSyncSameSourceAndBackupStorage(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.Given(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()

	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	// when:
	_, _, err := sourceStorageManager.SyncToWriter(t.Context(), sourceProvider)

	// then:
	require.Error(t, err)
}
