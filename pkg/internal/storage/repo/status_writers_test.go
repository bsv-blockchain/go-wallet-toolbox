package repo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	storageentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// The status writers must never silently no-op. A zero-row UPDATE — because the
// row is absent, the skip-list excluded it, or an expected-status precondition
// failed — returns a %w-wrapped repo.ErrStatusUpdateSkipped, and history notes
// are written only for transitions that actually matched a row (W1-4).

func TestUpdateKnownTxStatus_ZeroRows_ReturnsSkipped(t *testing.T) {
	// given: no known tx row exists for this txID.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ctx := t.Context()

	txID := "1111111111111111111111111111111111111111111111111111111111111111"

	// when:
	err := repos.UpdateKnownTxStatus(ctx, txID, wdk.ProvenTxStatusUnsent, nil, nil)

	// then:
	require.ErrorIs(t, err, repo.ErrStatusUpdateSkipped)
}

func TestUpdateKnownTxStatus_SkipListExcludes_ReturnsSkippedAndWritesNoNotes(t *testing.T) {
	// given: a completed known tx whose status is in the skip-list.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ctx := t.Context()

	txID := "2222222222222222222222222222222222222222222222222222222222222222"
	require.NoError(t, db.DB.Create(&models.KnownTx{
		TxID:   txID,
		Status: wdk.ProvenTxStatusCompleted,
	}).Error)

	note := history.NewBuilder().GetMerklePathNotFound("test-service")

	// when: we try to move it to unsent but skip anything already completed.
	err := repos.UpdateKnownTxStatus(
		ctx,
		txID,
		wdk.ProvenTxStatusUnsent,
		[]wdk.ProvenTxReqStatus{wdk.ProvenTxStatusCompleted},
		[]history.Builder{note},
	)

	// then: skipped, status unchanged, and no history note was persisted.
	require.ErrorIs(t, err, repo.ErrStatusUpdateSkipped)

	var reloaded models.KnownTx
	require.NoError(t, db.DB.First(&reloaded, "tx_id = ?", txID).Error)
	assert.Equal(t, wdk.ProvenTxStatusCompleted, reloaded.Status,
		"status must be unchanged when the skip-list excludes the row")

	var noteCount int64
	require.NoError(t, db.DB.Model(&models.TxNote{}).Where("tx_id = ?", txID).Count(&noteCount).Error)
	assert.Zero(t, noteCount, "no history note must be written when the update matched zero rows")
}

func TestUpdateTransactionStatusByTxID_ExpectedMismatch_Skipped(t *testing.T) {
	// given: a completed transaction whose current status is NOT the expected one.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "status-writer-mismatch", "test-storage")
	require.NoError(t, err)

	txID := "3333333333333333333333333333333333333333333333333333333333333333"
	const ref = "status-writer-mismatch-tx"
	require.NoError(t, repos.CreateTransaction(ctx, &storageentity.NewTx{
		UserID:    user.ID,
		Status:    wdk.TxStatusCompleted,
		Reference: ref,
		Version:   1,
		TxID:      &txID,
	}))

	// when: move to sending but only if currently unprocessed (it is completed).
	err = repos.UpdateTransactionStatusByTxID(ctx, txID, wdk.TxStatusSending, wdk.TxStatusUnprocessed)

	// then: skipped, status unchanged.
	require.ErrorIs(t, err, repo.ErrStatusUpdateSkipped)

	reloaded, err := repos.FindTransactionByReference(ctx, user.ID, ref)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Equal(t, wdk.TxStatusCompleted, reloaded.Status,
		"status must be unchanged when the expected-current precondition fails")
}

func TestUpdateTransactionStatusByTxID_ExpectedMatch_Updates(t *testing.T) {
	// given: a completed transaction; expected-current includes its status.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "status-writer-match", "test-storage")
	require.NoError(t, err)

	txID := "5555555555555555555555555555555555555555555555555555555555555555"
	const ref = "status-writer-match-tx"
	require.NoError(t, repos.CreateTransaction(ctx, &storageentity.NewTx{
		UserID:    user.ID,
		Status:    wdk.TxStatusCompleted,
		Reference: ref,
		Version:   1,
		TxID:      &txID,
	}))

	// when: move to sending, expecting the current completed status.
	err = repos.UpdateTransactionStatusByTxID(ctx, txID, wdk.TxStatusSending, wdk.TxStatusCompleted)

	// then: updated.
	require.NoError(t, err)

	reloaded, err := repos.FindTransactionByReference(ctx, user.ID, ref)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Equal(t, wdk.TxStatusSending, reloaded.Status,
		"status must be updated when the expected-current precondition matches")
}

func TestUpdateTransactionStatusByID_ZeroRows_Skipped(t *testing.T) {
	// given: no transaction row exists for this ID.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ctx := t.Context()

	// when:
	err := repos.UpdateTransactionStatusByID(ctx, 987654321, wdk.TxStatusFailed)

	// then:
	require.ErrorIs(t, err, repo.ErrStatusUpdateSkipped)
}

func TestInvalidateMerkleProofs_HappyPath_ReturnsCount(t *testing.T) {
	// Regression guard for the one-line `return affected, err` fix: the happy
	// path must still report the number of invalidated records and flip status
	// to reorg. (The txn-error propagation itself is hard to inject and is
	// covered by review.)
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ctx := t.Context()

	blockHash := "000000000000000000000000000000000000000000000000000000000000dead"
	merkleRoot := "beef"
	blockHeight := uint32(2000)
	require.NoError(t, db.DB.Create(&models.KnownTx{
		TxID:        "4444444444444444444444444444444444444444444444444444444444444444",
		Status:      wdk.ProvenTxStatusCompleted,
		BlockHash:   &blockHash,
		BlockHeight: &blockHeight,
		MerkleRoot:  &merkleRoot,
		MerklePath:  []byte{0x01, 0x02},
	}).Error)

	// when:
	affected, err := repos.InvalidateMerkleProofsByBlockHash(ctx, []string{blockHash})

	// then:
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	var reloaded models.KnownTx
	require.NoError(t, db.DB.First(&reloaded, "tx_id = ?", "4444444444444444444444444444444444444444444444444444444444444444").Error)
	assert.Equal(t, wdk.ProvenTxStatusReorg, reloaded.Status)
	assert.Nil(t, reloaded.BlockHash)
}
