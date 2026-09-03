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
	// that table gains a row per tag per output for the life of the wallet.
	//
	// Column order follows the predicate: the equality columns first, then
	// output_id so the subquery is answered from the index alone. Partial on
	// deleted_at because the soft delete appends `deleted_at IS NULL` to every
	// read.
	OutputID  uint   `gorm:"primaryKey;index:idx_output_tags_name_user,priority:3,where:deleted_at IS NULL"`
	TagName   string `gorm:"primaryKey;index:idx_output_tags_name_user,priority:1,where:deleted_at IS NULL"`
	TagUserID int    `gorm:"primaryKey;index:idx_output_tags_name_user,priority:2,where:deleted_at IS NULL"`
}
