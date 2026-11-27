package repo_test

import (
	"testing"
	"time"

	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// broadcast_at starts the proof wait. It is set when the wallet first claims a tx for a
// post, kept by later status writes, cleared by a requeue for rebroadcast (the next claim
// sets it again) and reset by a reorg.

func TestBroadcastAtLifecycle(t *testing.T) {
	// given: a tx waiting to be posted.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ctx := t.Context()

	txID := "4444444444444444444444444444444444444444444444444444444444444444"
	require.NoError(t, db.DB.Create(&models.KnownTx{TxID: txID, Status: wdk.ProvenTxStatusUnsent}).Error)

	broadcastAt := func() *time.Time {
		var row models.KnownTx
		require.NoError(t, db.DB.First(&row, "tx_id = ?", txID).Error)
		return row.BroadcastAt
	}
	require.Nil(t, broadcastAt())

	// when: the tx is claimed for its first post.
	claimed, err := repos.ClaimKnownTxsForBroadcast(ctx, []string{txID})
	require.NoError(t, err)
	require.Equal(t, []string{txID}, claimed)

	// then: the proof wait starts now.
	first := broadcastAt()
	require.NotNil(t, first)
	assert.WithinDuration(t, time.Now(), *first, time.Minute)

	// when: the post is accepted later.
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, repos.UpdateKnownTxStatus(ctx, txID, wdk.ProvenTxStatusUnmined, nil, nil))

	// then: the first value is kept.
	require.NotNil(t, broadcastAt())
	assert.True(t, first.Equal(*broadcastAt()), "a later broadcast-status write must keep the first broadcast_at")

	// when: the proof times out and the tx is requeued for a rebroadcast.
	require.NoError(t, repos.IncreaseKnownTxAttemptsForTxIDs(ctx, []string{txID}))
	updated, err := repos.ApplyProofTimeouts(ctx, 1, 0, []wdk.ProvenTxReqStatus{wdk.ProvenTxStatusUnmined})
	require.NoError(t, err)
	require.Len(t, updated, 1)
	require.Equal(t, wdk.ProvenTxStatusUnsent, updated[0].Status)

	// then: the wait is cleared...
	require.Nil(t, broadcastAt())

	// and: the next claim starts it again.
	_, err = repos.ClaimKnownTxsForBroadcast(ctx, []string{txID})
	require.NoError(t, err)
	require.NotNil(t, broadcastAt())
	assert.True(t, broadcastAt().After(*first), "the rebroadcast claim must restart the proof wait")
}

func TestBroadcastAtResetByReorg(t *testing.T) {
	// given: a mined tx broadcast a long time ago.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

	txID := "5555555555555555555555555555555555555555555555555555555555555555"
	const blockHash = "000000000000000000000000000000000000000000000000000000000000abcd"
	longAgo := time.Now().Add(-24 * time.Hour)
	require.NoError(t, db.DB.Create(&models.KnownTx{
		TxID:        txID,
		Status:      wdk.ProvenTxStatusCompleted,
		BlockHash:   to.Ptr(blockHash),
		BroadcastAt: &longAgo,
	}).Error)

	// when: its block is orphaned.
	_, err := repos.InvalidateMerkleProofsByBlockHash(t.Context(), []string{blockHash})
	require.NoError(t, err)

	// then: the proof wait restarts, so the SSE re-proof gets its window first.
	var row models.KnownTx
	require.NoError(t, db.DB.First(&row, "tx_id = ?", txID).Error)
	require.Equal(t, wdk.ProvenTxStatusReorg, row.Status)
	require.NotNil(t, row.BroadcastAt)
	assert.WithinDuration(t, time.Now(), *row.BroadcastAt, time.Minute)
}

func TestFindKnownTxIDsReadyForStatusSyncCutoff(t *testing.T) {
	// given: unmined txs posted before and after the cutoff, and one this wallet never posted.
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

	cutoff := time.Now().Add(-time.Hour)
	before := cutoff.Add(-time.Minute)
	after := cutoff.Add(time.Minute)

	const (
		postedBefore = "6666666666666666666666666666666666666666666666666666666666666666"
		postedAfter  = "7777777777777777777777777777777777777777777777777777777777777777"
		neverPosted  = "8888888888888888888888888888888888888888888888888888888888888888"
	)
	require.NoError(t, db.DB.Create(&[]models.KnownTx{
		{TxID: postedBefore, Status: wdk.ProvenTxStatusUnmined, BroadcastAt: &before},
		{TxID: postedAfter, Status: wdk.ProvenTxStatusUnmined, BroadcastAt: &after},
		{TxID: neverPosted, Status: wdk.ProvenTxStatusUnmined},
	}).Error)

	// when:
	rows, err := repos.FindKnownTxIDsReadyForStatusSync(t.Context(), []wdk.ProvenTxReqStatus{wdk.ProvenTxStatusUnmined}, 0, &cutoff)
	require.NoError(t, err)

	// then: the tx posted after the cutoff still waits; the others are overdue.
	txIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		txIDs = append(txIDs, row.TxID)
	}
	assert.ElementsMatch(t, []string{postedBefore, neverPosted}, txIDs)
}
