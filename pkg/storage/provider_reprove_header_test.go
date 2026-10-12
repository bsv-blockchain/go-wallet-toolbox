package storage_test

import (
	"testing"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	orphanedBlockHash = "00000000000000000000000000000000000000000000000000000000deadbeef"
	// WillReturnWithMindedTx serves the proof at this height.
	reprovedBlockHeight = 2000
)

// givenTxProvenInOrphanedBlock returns a tx whose stored proof points at orphanedBlockHash,
// while the services serve its proof in testservices.TestBlockHash.
func givenTxProvenInOrphanedBlock(t *testing.T, given testabilities.StorageFixture) (*storage.Provider, string) {
	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	txSpec, _ := given.Faucet(activeStorage, testusers.Alice).TopUp(100)
	txID := txSpec.ID().String()

	givenProvider.ARC().WhenQueryingTx(txID).WillReturnWithMindedTx()
	givenProvider.WhatsOnChain().OnTipBlockHeaderWillRespondWithOneElementList()
	givenProvider.WhatsOnChain().WillRespondOnTxStatus(200, testservices.TxStatusExpectation{
		ExpectBlockHash:   testservices.TestBlockHash,
		ExpectBlockHeight: int64(testservices.TestBlockHeight),
	})
	_, err := activeStorage.SynchronizeTransactionStatuses(t.Context())
	require.NoError(t, err)

	err = activeStorage.Database.DB.Model(&models.KnownTx{}).
		Where("tx_id = ?", txID).
		Update("block_hash", orphanedBlockHash).Error
	require.NoError(t, err)

	return activeStorage, txID
}

func givenMerkleRootVerification(t *testing.T, given testabilities.StorageFixture, txID, response string) {
	merklePath := testutils.MockValidMerklePath(t, txID, reprovedBlockHeight)
	merkleRoot, err := merklePath.ComputeRootHex(&txID)
	require.NoError(t, err)
	given.Provider().BHS().OnMerkleRootVerifyResponse(reprovedBlockHeight, merkleRoot, response)
}

func TestReproveHeader_UpdatesProofMovedToActiveBlock(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage, txID := givenTxProvenInOrphanedBlock(t, given)
	givenMerkleRootVerification(t, given, txID, testabilities.BHSMerkleRootConfirmed)

	// when:
	result, err := activeStorage.ReproveHeader(t.Context(), orphanedBlockHash)

	// then:
	require.NoError(t, err)
	require.Empty(t, result.Unchanged)
	require.Empty(t, result.Unavailable)
	require.Len(t, result.Updated, 1)
	require.Equal(t, txID, result.Updated[0].TxID)
	require.Equal(t, wdk.ProvenTxStatusCompleted, result.Updated[0].Status)
	require.Equal(t, testservices.TestBlockHash, result.Updated[0].BlockHash)
	require.Equal(t, uint32(reprovedBlockHeight), result.Updated[0].BlockHeight)

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined().
		WithBlockHash(to.Ptr(testservices.TestBlockHash))
}

func TestReproveHeader_UnchangedWhenServicesStillServeOrphanedBlock(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given: the stored proof and the one served by services both point at TestBlockHash
	activeStorage, txID := givenTxProvenInOrphanedBlock(t, given)
	err := activeStorage.Database.DB.Model(&models.KnownTx{}).
		Where("tx_id = ?", txID).
		Update("block_hash", testservices.TestBlockHash).Error
	require.NoError(t, err)

	// when:
	result, err := activeStorage.ReproveHeader(t.Context(), testservices.TestBlockHash)

	// then:
	require.NoError(t, err)
	require.Empty(t, result.Updated)
	require.Empty(t, result.Unavailable)
	require.Equal(t, []string{txID}, result.Unchanged)

	// and: the original proof is retained
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined().
		WithBlockHash(to.Ptr(testservices.TestBlockHash))
}

func TestReproveHeader_UnavailableWhenRootIsNotValid(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage, txID := givenTxProvenInOrphanedBlock(t, given)
	givenMerkleRootVerification(t, given, txID, "INVALID")

	// when:
	result, err := activeStorage.ReproveHeader(t.Context(), orphanedBlockHash)

	// then:
	require.NoError(t, err)
	require.Empty(t, result.Updated)
	require.Empty(t, result.Unchanged)
	require.Equal(t, []string{txID}, result.Unavailable)

	// and: the original proof is retained
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).
		WithStatus(wdk.ProvenTxStatusCompleted).
		IsMined().
		WithBlockHash(to.Ptr(orphanedBlockHash))
}

func TestReproveHeader_UnavailableWhenNoProof(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage, txID := givenTxProvenInOrphanedBlock(t, given)
	given.Provider().ARC().WhenQueryingTx(txID).WillReturnTransactionWithoutMerklePath()

	// when:
	result, err := activeStorage.ReproveHeader(t.Context(), orphanedBlockHash)

	// then:
	require.NoError(t, err)
	require.Empty(t, result.Updated)
	require.Empty(t, result.Unchanged)
	require.Equal(t, []string{txID}, result.Unavailable)

	// and:
	testabilities.ThenDBState(t, activeStorage).
		HasKnownTX(txID).
		WithBlockHash(to.Ptr(orphanedBlockHash))
}

func TestReproveHeader_NoTxsInOrphanedBlock(t *testing.T) {
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// given:
	activeStorage := given.Provider().GORM()

	// when:
	result, err := activeStorage.ReproveHeader(t.Context(), orphanedBlockHash)

	// then:
	require.NoError(t, err)
	require.Empty(t, result.Updated)
	require.Empty(t, result.Unchanged)
	require.Empty(t, result.Unavailable)
}
