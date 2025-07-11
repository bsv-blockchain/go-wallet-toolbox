package models

import (
	"fmt"
	"gorm.io/gorm"
	"time"
)

type OutputTag struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	OutputID  uint   `gorm:"primary_key"`
	TagName   string `gorm:"primary_key"`
	TagUserID int    `gorm:"primary_key"`
}

func (o *OutputTag) AfterDelete(tx *gorm.DB) (err error) {
	if o.DeletedAt.Valid {
		err = tx.Model(&OutputTag{}).
			Where("output_id = ? AND tag_name = ? AND tag_user_id = ?", o.OutputID, o.TagName, o.TagUserID).
			Update("updated_at", o.DeletedAt.Time).
			Error
		if err != nil {
			return fmt.Errorf("error deleting output_tag: %w", err)
		}
	}

	return
}
