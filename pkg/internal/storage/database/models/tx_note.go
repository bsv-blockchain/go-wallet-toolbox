package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type TxNote struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	TxID   string `gorm:"type:varchar(64);index;not null"`
	UserID *int   `gorm:"index"` // Nullable, can be used for user-specific events

	What       string `gorm:"not null"`
	Attributes datatypes.JSONMap
}

func (it *TxNote) ToWDKMap() map[string]any {
	count := len(it.Attributes) + 2 // +2 for "when" and "what"
	if it.UserID != nil {
		count++
	}

	note := make(map[string]any, count)
	note["when"] = it.CreatedAt
	note["what"] = it.What
	if it.UserID != nil {
		note["user_id"] = *it.UserID
	}
	for k, v := range it.Attributes {
		note[k] = v
	}
	return note
}
