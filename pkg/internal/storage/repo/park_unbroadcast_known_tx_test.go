package repo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// ParkUnbroadcastKnownTx is the guard of the pre-broadcast abort: it may only fire while the
// transaction provably never reached a broadcaster. Parking one that was (or may have been)
// posted would let its inputs be released while the network still holds the transaction.

func TestParkUnbroadcastKnownTx_NeverPosted_Parks(t *testing.T) {
	tests := map[string]wdk.ProvenTxReqStatus{
		"unprocessed": wdk.ProvenTxStatusUnprocessed,
		"unsent":      wdk.ProvenTxStatusUnsent,
		"nosend":      wdk.ProvenTxStatusNoSend,
	}

	for name, status := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			db, cleanup := dbfixtures.TestDatabase(t)
			defer cleanup()

			repos := repo.NewSQLRepositories(db.DB)
			txID := "1111111111111111111111111111111111111111111111111111111111111111"
			require.NoError(t, db.DB.Create(&models.KnownTx{
				TxID:         txID,
				Status:       status,
				WasBroadcast: false,
				Attempts:     0,
			}).Error)

			note := history.NewBuilder().AbortBeforeBroadcast(wdk.ProvenTxStatusInvalid)

			// when:
			applied, err := repos.ParkUnbroadcastKnownTx(t.Context(), txID, []history.Builder{note})

			// then:
			require.NoError(t, err)
			assert.True(t, applied)

			var reloaded models.KnownTx
			require.NoError(t, db.DB.First(&reloaded, "tx_id = ?", txID).Error)
			assert.Equal(t, wdk.ProvenTxStatusInvalid, reloaded.Status)

			var noteCount int64
			require.NoError(t, db.DB.Model(&models.TxNote{}).Where("tx_id = ?", txID).Count(&noteCount).Error)
			assert.EqualValues(t, 1, noteCount)
		})
	}
}

func TestParkUnbroadcastKnownTx_PostedOrPosting_IsRefused(t *testing.T) {
	tests := map[string]models.KnownTx{
		"already submitting": {
			Status:       wdk.ProvenTxStatusSending,
			WasBroadcast: true,
		},
		"was broadcast flag set": {
			Status:       wdk.ProvenTxStatusUnsent,
			WasBroadcast: true,
		},
		"has a recorded attempt": {
			Status:   wdk.ProvenTxStatusUnsent,
			Attempts: 1,
		},
		"accepted by the network": {
			Status:       wdk.ProvenTxStatusUnmined,
			WasBroadcast: true,
			Attempts:     1,
		},
	}

	for name, knownTx := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			db, cleanup := dbfixtures.TestDatabase(t)
			defer cleanup()

			repos := repo.NewSQLRepositories(db.DB)
			txID := "2222222222222222222222222222222222222222222222222222222222222222"
			knownTx.TxID = txID
			require.NoError(t, db.DB.Create(&knownTx).Error)

			note := history.NewBuilder().AbortBeforeBroadcast(wdk.ProvenTxStatusInvalid)

			// when:
			applied, err := repos.ParkUnbroadcastKnownTx(t.Context(), txID, []history.Builder{note})

			// then:
			require.NoError(t, err)
			assert.False(t, applied, "a transaction that may have reached the network must not be parked")

			var reloaded models.KnownTx
			require.NoError(t, db.DB.First(&reloaded, "tx_id = ?", txID).Error)
			assert.Equal(t, knownTx.Status, reloaded.Status, "status must be unchanged")

			var noteCount int64
			require.NoError(t, db.DB.Model(&models.TxNote{}).Where("tx_id = ?", txID).Count(&noteCount).Error)
			assert.Zero(t, noteCount, "no history note must be written when nothing was parked")
		})
	}
}

func TestParkUnbroadcastKnownTx_MissingRow_IsRefused(t *testing.T) {
	// given: no known tx row at all
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB)

	// when:
	applied, err := repos.ParkUnbroadcastKnownTx(t.Context(), "3333333333333333333333333333333333333333333333333333333333333333", nil)

	// then:
	require.NoError(t, err)
	assert.False(t, applied)
}
