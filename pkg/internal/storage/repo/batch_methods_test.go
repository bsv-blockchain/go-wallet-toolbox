package repo_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	storageentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// batchSizes covers the default (single-row), a size smaller than the input (several statements)
// and one larger than the input (one statement).
var batchSizes = []int{0, 3, 1000}

func withBatchSize(size int) dbfixtures.DBConfigModifier {
	return func(c *defs.Database) { c.InsertBatchSize = size }
}

func TestUpsertKnownTxs_AllBatchSizesGiveTheSameRows(t *testing.T) {
	var want []knownTxSnapshot
	for _, size := range batchSizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			db, cleanup := dbfixtures.TestDatabase(t, withBatchSize(size))
			defer cleanup()
			repos := db.CreateRepositories()
			ctx := t.Context()

			mk := func(i int, status wdk.ProvenTxReqStatus, skip ...wdk.ProvenTxReqStatus) *storageentity.UpsertKnownTx {
				return &storageentity.UpsertKnownTx{
					TxID: fmt.Sprintf("%064x", i), Status: status, RawTx: []byte{byte(i)}, SkipForStatuses: skip,
				}
			}
			note := history.NewBuilder().ProcessAction(1)

			// first call inserts 8 rows, the second updates them; one row is guarded by a skip list
			first := make([]*storageentity.UpsertKnownTx, 8)
			for i := range first {
				first[i] = mk(i, wdk.ProvenTxStatusUnsent)
			}
			require.NoError(t, repos.UpsertKnownTxs(ctx, first, note))

			second := make([]*storageentity.UpsertKnownTx, 8)
			for i := range second {
				second[i] = mk(i, wdk.ProvenTxStatusSending)
			}
			require.NoError(t, repos.UpsertKnownTxs(ctx, second, note))

			require.NoError(t, db.DB.Model(&models.KnownTx{}).Where("tx_id = ?", fmt.Sprintf("%064x", 3)).
				Update("status", wdk.ProvenTxStatusCompleted).Error)
			require.NoError(t, repos.UpsertKnownTxs(ctx, []*storageentity.UpsertKnownTx{
				mk(3, wdk.ProvenTxStatusUnsent, wdk.ProvenTxStatusCompleted), mk(4, wdk.ProvenTxStatusUnmined),
			}, note))

			// duplicate tx ids in one call are applied in order
			require.NoError(t, repos.UpsertKnownTxs(ctx, []*storageentity.UpsertKnownTx{
				mk(9, wdk.ProvenTxStatusUnsent), mk(9, wdk.ProvenTxStatusSending),
			}, note))

			var got []knownTxSnapshot
			for i := range 10 {
				if snap, ok := snapshotKnownTx(t, db.DB, fmt.Sprintf("%064x", i)); ok {
					snap.row.CreatedAt, snap.row.UpdatedAt = zeroTime(), zeroTime()
					for j := range snap.notes {
						snap.notes[j].CreatedAt = zeroTime()
					}
					got = append(got, snap)
				}
			}
			require.Len(t, got, 9)
			assert.Equal(t, wdk.ProvenTxStatusCompleted, got[3].row.Status, "guarded row untouched")
			assert.Len(t, got[3].notes, 2, "guarded row got no third note")
			assert.Equal(t, wdk.ProvenTxStatusSending, got[8].row.Status)

			if want == nil {
				want = got
			} else {
				assert.Equal(t, want, got)
			}
		})
	}
}

func TestCreateTransactions_AllBatchSizesGiveTheSameRows(t *testing.T) {
	type dump struct {
		Txs     []models.Transaction
		Outputs []models.Output
		UTXOs   []models.UserUTXO
	}
	var want *dump
	for _, size := range batchSizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			f := newBasketFixtureWith(t, withBatchSize(size))
			ctx := t.Context()
			user, err := f.repos.CreateUser(ctx, "batch-tx-user", "s", wdk.DefaultBasketConfiguration())
			require.NoError(t, err)

			newTxs := make([]*storageentity.NewTx, 7)
			for i := range newTxs {
				newTxs[i] = f.newTx(user.ID, "custom")
				newTxs[i].Reference = fmt.Sprintf("batch-ref-%d", i)
				newTxs[i].Outputs[0].Change = true
				newTxs[i].Outputs[0].Purpose = wdk.ChangePurpose
				newTxs[i].Outputs[0].ProvidedBy = wdk.ProvidedByStorage
				newTxs[i].UTXOStatus = wdk.UTXOStatusMined
			}
			var txInserts int
			require.NoError(t, f.db.Callback().Create().Before("gorm:create").Register("count_tx_inserts", func(d *gorm.DB) {
				if d.Statement.Table == "bsv_transactions" {
					txInserts++
				}
			}))
			require.NoError(t, f.repos.CreateTransactions(ctx, newTxs))
			wantInserts := map[int]int{0: 7, 3: 3, 1000: 1}[size]
			assert.Equal(t, wantInserts, txInserts, "INSERT statements into bsv_transactions")

			got := &dump{}
			require.NoError(t, f.db.Order("reference").Find(&got.Txs).Error)
			require.NoError(t, f.db.Order("transaction_id, vout").Find(&got.Outputs).Error)
			require.NoError(t, f.db.Order("output_id").Find(&got.UTXOs).Error)
			require.Len(t, got.Txs, 7)
			require.Len(t, got.Outputs, 7)
			require.Len(t, got.UTXOs, 7)
			for i := range got.Txs {
				got.Txs[i].CreatedAt, got.Txs[i].UpdatedAt = zeroTime(), zeroTime()
			}
			for i := range got.Outputs {
				got.Outputs[i].CreatedAt, got.Outputs[i].UpdatedAt = zeroTime(), zeroTime()
			}
			for i := range got.UTXOs {
				got.UTXOs[i].CreatedAt = zeroTime()
			}

			if want == nil {
				want = got
			} else {
				assert.Equal(t, want, got)
			}
		})
	}
}

func zeroTime() time.Time { return time.Time{} }

// With a small batch size the outputs of one transaction are inserted by several statements; they
// must still share created_at, which the sync chunk ordering (created_at DESC, id) relies on.
func TestCreateTransaction_BatchedOutputsShareCreatedAt(t *testing.T) {
	f := newBasketFixtureWith(t, withBatchSize(2))
	ctx := t.Context()
	user, err := f.repos.CreateUser(ctx, "batch-ts-user", "s", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	newTx := f.newTx(user.ID, "custom")
	first := newTx.Outputs[0]
	for vout := uint32(1); vout < 5; vout++ {
		out := *first
		out.Vout = vout
		newTx.Outputs = append(newTx.Outputs, &out)
	}
	require.NoError(t, f.repos.CreateTransaction(ctx, newTx))

	var outputs []models.Output
	require.NoError(t, f.db.Order("vout").Find(&outputs).Error)
	require.Len(t, outputs, 5)
	for _, o := range outputs {
		assert.True(t, o.CreatedAt.Equal(outputs[0].CreatedAt), "vout %d created_at differs", o.Vout)
	}
}
