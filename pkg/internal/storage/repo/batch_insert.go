package repo

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// InsertBatchSize reports the configured rows-per-INSERT for multi-row inserts on db
// (defs.Database.InsertBatchSize, applied as gorm.Config.CreateBatchSize). 0 means batching is
// off: the bulk helpers keep inserting one row per statement.
func InsertBatchSize(db *gorm.DB) int {
	return db.CreateBatchSize
}

// createRows inserts rows through tx. With batching off (size 0) it issues one single-row INSERT
// per element, exactly like the loops it replaces; with a batch size it issues multi-row INSERTs
// of at most that many rows (gorm CreateInBatches).
func createRows[T any](tx *gorm.DB, rows []*T) error {
	if len(rows) == 0 {
		return nil
	}

	if size := InsertBatchSize(tx); size > 0 {
		if err := tx.CreateInBatches(rows, size).Error; err != nil {
			return fmt.Errorf("batched insert of %d rows failed: %w", len(rows), err)
		}

		return nil
	}

	for _, row := range rows {
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("single-row insert failed: %w", err)
		}
	}

	return nil
}

// withStableNow makes every statement issued through the returned handle stamp created_at and
// updated_at with the same instant. When a multi-row insert is split into batches, each batch would
// otherwise call the clock again, so rows of one logical insert (the outputs of one transaction,
// say) would no longer share a timestamp, which changes created_at based orderings such as the sync
// chunk order. With batching off the handle is returned unchanged.
func withStableNow(tx *gorm.DB) *gorm.DB {
	if InsertBatchSize(tx) <= 0 {
		return tx
	}

	now := tx.NowFunc()

	return tx.Session(&gorm.Session{NowFunc: func() time.Time { return now }})
}
