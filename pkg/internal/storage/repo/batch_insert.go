package repo

import (
	"fmt"

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
