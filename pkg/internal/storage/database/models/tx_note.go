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
