package models

import "time"

// Timestamps matches TS addTimeStamps(): snake_case created_at / updated_at, no deleted_at.
// Every model embeds this instead of gorm.Model, which injects a generic `id` and `deleted_at`
// that violate the TS schema contract.
//
// Models declare their own per-table named PK explicitly, e.g.:
//
//	OutputID uint `gorm:"column:outputId;primaryKey;autoIncrement"`
//
// Soft-deletable models (certificates, output_baskets, output_tags, output_tags_map,
// tx_labels, tx_labels_map) add:
//
//	IsDeleted bool `gorm:"column:isDeleted;not null;default:false"`
type Timestamps struct {
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}
