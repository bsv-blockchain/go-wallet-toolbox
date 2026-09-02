package models

import (
	"time"

	"gorm.io/gorm"
)

type OutputTag struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// idx_output_tags_name_user serves the tag filter, which selects output_id
	// by tag name:
	//
	//	WHERE tag_name IN (...) AND tag_user_id = ?
	//
	// The primary key cannot answer that - its leading column is output_id - so
	// without this the lookup falls back to scanning the whole join table, and
	// the table gains a row per tag per output forever. Measured on a wallet
	// after 2,000 trades: 19,571 sequential scans reading 92,989,035 tuples from
	// 8,000 rows, the single largest source of tuple reads in the schema.
	//
	// Column order follows the predicate: equality columns first, then output_id
	// so the subquery is answered from the index alone. Partial on deleted_at
	// because the soft delete appends `deleted_at IS NULL` to every read.
	OutputID  uint   `gorm:"primary_key;index:idx_output_tags_name_user,priority:3,where:deleted_at IS NULL"`
	TagName   string `gorm:"primary_key;index:idx_output_tags_name_user,priority:1,where:deleted_at IS NULL"`
	TagUserID int    `gorm:"primary_key;index:idx_output_tags_name_user,priority:2,where:deleted_at IS NULL"`
}
