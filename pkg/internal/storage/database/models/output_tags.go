package models

import (
	"time"

	"gorm.io/gorm"
)

type OutputTag struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	OutputID  uint   `gorm:"primary_key"`
	TagName   string `gorm:"primary_key"`
	TagUserID int    `gorm:"primary_key"`
}
