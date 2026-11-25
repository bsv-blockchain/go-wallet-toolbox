package repo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	storageentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type upsertFn func(tx *gorm.DB, req *storageentity.UpsertKnownTx, note history.Builder) error

type knownTxSnapshot struct {
	row   models.KnownTx
	notes []models.TxNote
}

func snapshotKnownTx(t *testing.T, db *gorm.DB, txID string) (knownTxSnapshot, bool) {
	t.Helper()

	var rows []models.KnownTx
	require.NoError(t, db.Find(&rows, "tx_id = ?", txID).Error)
	if len(rows) == 0 {
		return knownTxSnapshot{}, false
	}

	var notes []models.TxNote
	require.NoError(t, db.Order("id").Find(&notes, "tx_id = ?", txID).Error)
	for i := range notes {
		notes[i].ID = 0
	}

	return knownTxSnapshot{row: rows[0], notes: notes}, true
}

// TestUpsertKnownTx_SingleStatementMatchesReadModifyWrite runs the same scenario through the
// one-statement upsert and through the original SELECT/UPDATE/INSERT code, on two separate
// databases, and requires identical rows and notes (timestamps aside).
func TestUpsertKnownTx_SingleStatementMatchesReadModifyWrite(t *testing.T) {
	const txID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	type step struct {
		name string
		// seed mutates the row directly before the upsert (columns the upsert must preserve)
		seed func(db *gorm.DB)
		req  storageentity.UpsertKnownTx
	}

	steps := []step{
		{name: "insert new", req: storageentity.UpsertKnownTx{TxID: txID, Status: wdk.ProvenTxStatusUnprocessed, RawTx: []byte{1, 2}, InputBeef: []byte{3}}},
		{
			name: "update keeps untouched columns",
			seed: func(db *gorm.DB) {
				require.NoError(t, db.Model(&models.KnownTx{}).Where("tx_id = ?", txID).Updates(map[string]any{
					"attempts": 7, "notified": true, "notify": `{"a":1}`, "merkle_path": []byte{9}, "block_height": 5, "was_broadcast": false,
				}).Error)
			},
			req: storageentity.UpsertKnownTx{TxID: txID, Status: wdk.ProvenTxStatusSending, RawTx: []byte{4}, InputBeef: []byte{5}},
		},
		{name: "was_broadcast is sticky", req: storageentity.UpsertKnownTx{TxID: txID, Status: wdk.ProvenTxStatusUnsent, RawTx: []byte{6}}},
		{name: "nil raw tx and beef overwrite", req: storageentity.UpsertKnownTx{TxID: txID, Status: wdk.ProvenTxStatusUnprocessed}},
		{
			name: "skip for status writes nothing",
			seed: func(db *gorm.DB) {
				require.NoError(t, db.Model(&models.KnownTx{}).Where("tx_id = ?", txID).Update("status", wdk.ProvenTxStatusCompleted).Error)
			},
			req: storageentity.UpsertKnownTx{
				TxID: txID, Status: wdk.ProvenTxStatusUnsent, RawTx: []byte{7},
				SkipForStatuses: []wdk.ProvenTxReqStatus{wdk.ProvenTxStatusCompleted},
			},
		},
		{
			name: "skip list not matching still updates",
			req: storageentity.UpsertKnownTx{
				TxID: txID, Status: wdk.ProvenTxStatusUnmined, RawTx: []byte{8},
				SkipForStatuses: []wdk.ProvenTxReqStatus{wdk.ProvenTxStatusInvalid},
			},
		},
	}

	run := func(t *testing.T, fn upsertFn) []knownTxSnapshot {
		db, cleanup := dbfixtures.TestDatabase(t)
		defer cleanup()
		_ = repo.NewSQLRepositories(db.DB, dbretry.NoRetry())

		var out []knownTxSnapshot
		for _, s := range steps {
			if s.seed != nil {
				s.seed(db.DB)
			}
			req := s.req
			require.NoError(t, db.DB.Transaction(func(tx *gorm.DB) error {
				return fn(tx, &req, history.NewBuilder().ProcessAction(1))
			}), s.name)
			snap, ok := snapshotKnownTx(t, db.DB, txID)
			require.True(t, ok, s.name)
			out = append(out, snap)
		}
		return out
	}

	oldRes := run(t, repo.UpsertKnownTxReadModifyWrite)
	newRes := run(t, repo.UpsertKnownTxSingleStatement)

	for i, s := range steps {
		o, n := oldRes[i], newRes[i]
		// timestamps differ between runs; compare everything else
		o.row.CreatedAt, o.row.UpdatedAt, n.row.CreatedAt, n.row.UpdatedAt = time.Time{}, time.Time{}, time.Time{}, time.Time{}
		for j := range o.notes {
			o.notes[j].CreatedAt = time.Time{}
		}
		for j := range n.notes {
			n.notes[j].CreatedAt = time.Time{}
		}
		assert.Equal(t, o.row, n.row, "row after step %q", s.name)
		assert.Equal(t, o.notes, n.notes, "notes after step %q", s.name)
	}
}
