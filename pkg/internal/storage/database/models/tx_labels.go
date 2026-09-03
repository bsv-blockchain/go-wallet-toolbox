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
	// idx_output_tags_name_user: the label filter selects transaction_id by
	// label name, which the primary key cannot serve because it leads with
	// transaction_id.
	TransactionID uint   `gorm:"primaryKey;index:idx_transaction_labels_name_user,priority:3,where:deleted_at IS NULL"`
	LabelName     string `gorm:"primaryKey;index:idx_transaction_labels_name_user,priority:1,where:deleted_at IS NULL"`
	LabelUserID   int    `gorm:"primaryKey;index:idx_transaction_labels_name_user,priority:2,where:deleted_at IS NULL"`
}
