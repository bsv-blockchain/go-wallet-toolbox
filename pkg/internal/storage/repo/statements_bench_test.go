package repo_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	storageentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// BenchmarkUpsertKnownTx compares the original SELECT + UPDATE + INSERT implementation ("old")
// with the single-statement one ("new"), for a tx that does not exist yet and for one that does.
func BenchmarkUpsertKnownTx(b *testing.B) {
	impls := []struct {
		name string
		fn   func(*gorm.DB, *storageentity.UpsertKnownTx, history.Builder) error
	}{
		{"old", repo.UpsertKnownTxReadModifyWrite},
		{"new", repo.UpsertKnownTxSingleStatement},
	}
	note := history.NewBuilder().ProcessAction(1)

	for _, impl := range impls {
		b.Run("insert/"+impl.name, func(b *testing.B) {
			db, cleanup := dbfixtures.TestDatabase(b)
			defer cleanup()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req := &storageentity.UpsertKnownTx{TxID: fmt.Sprintf("%064x", i), Status: wdk.ProvenTxStatusUnsent, RawTx: []byte{1}}
				require.NoError(b, db.DB.Transaction(func(tx *gorm.DB) error { return impl.fn(tx, req, note) }))
			}
		})
		b.Run("update/"+impl.name, func(b *testing.B) {
			db, cleanup := dbfixtures.TestDatabase(b)
			defer cleanup()
			req := &storageentity.UpsertKnownTx{TxID: fmt.Sprintf("%064x", 1), Status: wdk.ProvenTxStatusUnsent, RawTx: []byte{1}}
			require.NoError(b, db.DB.Transaction(func(tx *gorm.DB) error { return impl.fn(tx, req, note) }))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				require.NoError(b, db.DB.Transaction(func(tx *gorm.DB) error { return impl.fn(tx, req, note) }))
			}
		})
	}
}

// BenchmarkInsertKnownTxs inserts rows per op with the default single-row path ("single",
// insert_batch_size 0) and with the batched path ("batch", insert_batch_size 1000).
func BenchmarkInsertKnownTxs(b *testing.B) {
	note := history.NewBuilder().ProcessAction(1)
	for _, rows := range []int{1, 10, 100, 1000} {
		for _, mode := range []struct {
			name string
			size int
		}{{"single", 0}, {"batch", 1000}} {
			b.Run(fmt.Sprintf("rows=%d/%s", rows, mode.name), func(b *testing.B) {
				db, cleanup := dbfixtures.TestDatabase(b, withBatchSize(mode.size))
				defer cleanup()
				repos := db.CreateRepositories()
				ctx := b.Context()

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					reqs := make([]*storageentity.UpsertKnownTx, rows)
					for r := range reqs {
						reqs[r] = &storageentity.UpsertKnownTx{
							TxID: fmt.Sprintf("%032x%032x", i, r), Status: wdk.ProvenTxStatusUnsent, RawTx: []byte{1, 2, 3},
						}
					}
					require.NoError(b, repos.UpsertKnownTxs(ctx, reqs, note))
				}
			})
		}
	}
}

// BenchmarkPrepareStatements runs a write and a read repository call with GORM PrepareStmt off and on.
func BenchmarkPrepareStatements(b *testing.B) {
	for _, prepare := range []bool{false, true} {
		name := "off"
		if prepare {
			name = "on"
		}
		modifier := func(c *defs.Database) { c.PrepareStatements = prepare }

		b.Run("write/"+name, func(b *testing.B) {
			db, cleanup := dbfixtures.TestDatabase(b, modifier)
			defer cleanup()
			repos := db.CreateRepositories()
			req := &storageentity.UpsertKnownTx{TxID: fmt.Sprintf("%064x", 1), Status: wdk.ProvenTxStatusUnsent, RawTx: []byte{1}}
			note := history.NewBuilder().ProcessAction(1)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				require.NoError(b, repos.UpsertKnownTx(b.Context(), req, note))
			}
		})
		b.Run("read/"+name, func(b *testing.B) {
			db, cleanup := dbfixtures.TestDatabase(b, modifier)
			defer cleanup()
			repos := db.CreateRepositories()
			txID := fmt.Sprintf("%064x", 1)
			require.NoError(b, repos.UpsertKnownTx(b.Context(), &storageentity.UpsertKnownTx{TxID: txID, Status: wdk.ProvenTxStatusUnsent}, history.NewBuilder().ProcessAction(1)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := repos.FindKnownTxStatuses(b.Context(), txID)
				require.NoError(b, err)
			}
		})
	}
}
