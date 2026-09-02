package models

import (
	"time"

	"gorm.io/gorm"
)

type TransactionLabel struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// idx_transaction_labels_name_user is the label-side twin of
	// idx_output_tags_name_user: the label filter selects transaction_id by label
	// name, which the primary key cannot serve because it leads with
	// transaction_id. Measured on the same wallet: 28,982 sequential scans, and
	// 154 tuples fetched per index scan where a selective lookup returns ~1.
	TransactionID uint   `gorm:"primary_key;index:idx_transaction_labels_name_user,priority:3,where:deleted_at IS NULL"`
	LabelName     string `gorm:"primary_key;index:idx_transaction_labels_name_user,priority:1,where:deleted_at IS NULL"`
	LabelUserID   int    `gorm:"primary_key;index:idx_transaction_labels_name_user,priority:2,where:deleted_at IS NULL"`
}
