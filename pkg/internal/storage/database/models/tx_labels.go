package models

import (
	"time"

	"gorm.io/gorm"
)

type TransactionLabel struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	TransactionID uint   `gorm:"primary_key"`
	LabelName     string `gorm:"primary_key"`
	LabelUserID   int    `gorm:"primary_key"`
}
