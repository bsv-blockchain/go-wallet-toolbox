package storage_test

import (
	"fmt"
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestSyncProcess(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.GivenSyncFixture(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()
	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	seed := givenSourceDB.SeedDB(sourceProvider, testusers.Alice)
	ownedMinedTx := seed.SetLabels(commonLabel, customLabelTx1).OwnsMinedTransaction()
	ownedTx := seed.SetLabels(commonLabel, customLabelTx2).OwnsTransaction()
	internalizedTxID, createActionResult := seed.OwnsInternalizedAndNotProcessedTx()

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	// when:
	_, err := sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 36, inserts)
	assert.Equal(t, 1, updates)

	// and:
	thenDBState := testabilities.ThenSync(t).DBState(backupProvider)

	// and knownTxs:
	thenDBState.HasKnownTX(ownedMinedTx.ID().String()).
		WithStatus(wdk.ProvenTxStatusCompleted).
		WithAttempts(0).
		HasRawTx().
		IsMined()

	thenDBState.HasKnownTX(ownedTx.ID().String()).
		WithStatus(wdk.ProvenTxStatusUnmined).
		HasRawTx()

	thenDBState.HasKnownTX(internalizedTxID).
		WithStatus(wdk.ProvenTxStatusUnmined).
		HasRawTx().
		TxNotes(func(then testabilities.TxNotesAssertion) {
			then.Count(1).
				Note("internalizeAction", to.Ptr(testusers.Alice.ID), nil)
		})

	// and user's transactions:
	thenDBState.
		HasUserTransactionByReference(testusers.Alice, fixtures.FaucetReference(ownedMinedTx.ID().String())).
		WithTxID(ownedMinedTx.ID().String()).
		WithStatus(wdk.TxStatusCompleted).
		WithLabels(commonLabel, customLabelTx1)

	thenDBState.
		HasUserTransactionByReference(testusers.Alice, fixtures.FaucetReference(ownedTx.ID().String())).
		WithTxID(ownedTx.ID().String()).
		WithStatus(wdk.TxStatusUnproven).
		WithLabels(commonLabel, customLabelTx2)

	thenDBState.
		HasUserTransactionByReference(testusers.Alice, createActionResult.Reference).
		WithoutTxID().
		WithStatus(wdk.TxStatusUnsigned).
		WithLabels(fixtures.CreateActionTestLabel)

	// and outputs:
	thenDBState.AllOutputs(testusers.Alice).
		WithCount(12).
		WithCountHavingTxID(3).
		WithCountHavingTags(3, fixtures.CreateActionTestTag).
		WithCountHavingTags(1, fixtures.FaucetTag(0)).
		WithCountHavingTags(1, fixtures.FaucetTag(1))

	thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).
		WithCount(11)

	// and:
	const fee = 1
	thenDBState.CanCreateActionForSatoshis(
		testusers.Alice,
		seed.GetAvailableBalance()-fee,
	)
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
	_, err := sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)
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
	_, err := sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, customBasketsCount, inserts)
	assert.Equal(t, 1, updates)
}

func TestSyncProcessWithManyTransactionsOnSeveralChunks(t *testing.T) {
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
	_, err := sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider, wdk.WithMaxSyncItems(maxItemsPerSingleSync))

	// then:
	require.NoError(t, err)

	knownTxCount := numberOfTxs
	userTxsCount := numberOfTxs
	outputsCount := numberOfTxs
	tagsCount := numberOfTxs + 1    // One for the "fixtures.CreateActionTestTag"
	tagMapsCount := 2 * numberOfTxs // Each transaction has two tags

	allCount := knownTxCount + userTxsCount + outputsCount + tagsCount + tagMapsCount
	assert.Equal(t, allCount, inserts)
	assert.Equal(t, 1, updates)

	// and known transactions:
	thenDBState := testabilities.ThenSync(t).DBState(backupProvider)
	thenDBState.HasKnownTXs(seed.GetAllOwnedTransactionIDs()...)

	// and outputs:
	thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).
		WithCount(numberOfTxs).
		WithCountHavingTxID(numberOfTxs)

	// and:
	const fee = 4 // NOTE: Minimum fee to cover so many UTXOs as inputs
	thenDBState.CanCreateActionForSatoshis(
		testusers.Alice,
		seed.GetAvailableBalance()-4*fee,
	)
}

func TestSyncProcessWithMergeUser(t *testing.T) {
	// given:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	// NOTE: Backup storage is created first, so the user data will be older than in the source storage - so the merge will happen
	backupProvider := givenBackupDB.Provider().GORM()

	// and:
	givenSourceDB, cleanup := testabilities.Given(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()

	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	// when:
	_, err := sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)
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
	_, err = sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)
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

func TestSyncProcessWithBasketsNumIDMissmatch(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.GivenSyncFixture(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()
	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Bob, sourceProvider)

	// and:
	// NOTE: This creates a situation where the source storage will generate basketID for Bob = 2, but the backup storage will have basketID = 1
	_, err := sourceProvider.GetSyncChunk(t.Context(), givenSourceDB.RequestSyncChunk(testusers.Alice).Args())
	require.NoError(t, err)

	seed := givenSourceDB.SeedDB(sourceProvider, testusers.Bob)
	_ = seed.OwnsTransaction()

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	// when:
	_, err = sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 7, inserts)
	assert.Equal(t, 1, updates)

	// and outputs:
	thenDBState := testabilities.ThenSync(t).DBState(backupProvider)
	thenDBState.Outputs(testusers.Bob, wdk.BasketNameForChange).
		WithCount(1).
		WithCountHavingTxID(1)
}

func TestSyncProcessWithRelinquishOutput(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.GivenSyncFixture(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()
	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	seed := givenSourceDB.SeedDB(sourceProvider, testusers.Alice)
	ownedTx := seed.OwnsTransaction()
	_ = ownedTx

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	// when:
	_, err := sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 7, inserts)
	assert.Equal(t, 1, updates)

	// when:
	err = sourceProvider.RelinquishOutput(t.Context(), testusers.Alice.AuthID(), wdk.RelinquishOutputArgs{
		Output: string(primitives.NewOutpointString(ownedTx.ID().String(), 0)),
	})
	require.NoError(t, err)

	// and:
	inserts, updates, err = sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	require.NoError(t, err)
	assert.Equal(t, 0, inserts)
	assert.Equal(t, 1, updates)

	// and outputs:
	thenDBState := testabilities.ThenSync(t).DBState(backupProvider)

	thenDBState.AllOutputs(testusers.Alice).
		WithCount(1)

	thenDBState.Outputs(testusers.Alice, wdk.BasketNameForChange).
		WithCount(0) // NOTE: Relinquished output is not in the change basket anymore
}

func TestSyncProcessWhenLabelAndTagChanges(t *testing.T) {
	// given:
	givenSourceDB, cleanup := testabilities.GivenSyncFixture(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().WithRandomizer(randomizer.NewTestRandomizer()).GORM()
	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	const (
		label1    = "label1"
		label2    = "label2"
		tag1      = "tag1"
		tag2      = "tag2"
		reference = "YWFhYWFhYWFhYWFh"
	)

	// and:
	internalizeArgs, _ := givenSourceDB.Action(sourceProvider).PreInternalized()
	internalizeArgs.Labels = []primitives.StringUnder300{commonLabel, label1}
	internalizeArgs.Description = "first internalize"
	internalizeArgs.Outputs = []*wdk.InternalizeOutput{
		{
			OutputIndex: 0,
			Protocol:    wdk.BasketInsertionProtocol,
			InsertionRemittance: &wdk.BasketInsertion{
				Basket: "custom_basket",
				Tags:   []primitives.StringUnder300{tag1},
			},
		},
	}
	// and:
	_, err := sourceProvider.InternalizeAction(t.Context(), testusers.Alice.AuthID(), *internalizeArgs)
	require.NoError(t, err)

	// and:
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()

	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	// when:
	_, err = sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)
	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	// then:
	require.NoError(t, err)
	assert.Equal(t, 10, inserts)
	assert.Equal(t, 1, updates)

	// and:
	thenDBState := testabilities.ThenSync(t).DBState(backupProvider)
	thenDBState.HasUserTransactionByReference(testusers.Alice, reference).
		WithLabels(commonLabel, label1)

	thenDBState.AllOutputs(testusers.Alice).WithCountHavingTags(1, tag1)

	// when:
	internalizeArgs.Labels = []primitives.StringUnder300{commonLabel, label2}
	internalizeArgs.Outputs[0].InsertionRemittance.Tags = []primitives.StringUnder300{tag2}
	_, err = sourceProvider.InternalizeAction(t.Context(), testusers.Alice.AuthID(), *internalizeArgs)
	require.NoError(t, err)

	// then:
	require.NoError(t, err)

	// and:
	inserts, updates, err = sourceStorageManager.SyncToWriter(t.Context(), backupProvider)

	require.NoError(t, err)
	assert.Equal(t, 4, inserts)
	assert.Equal(t, 3, updates)

	// and:
	thenDBState.HasUserTransactionByReference(testusers.Alice, reference).
		WithLabels(commonLabel, label1, label2)

	thenDBState.AllOutputs(testusers.Alice).
		WithCountHavingTags(0, tag1).
		WithCountHavingTags(1, tag2)
}
