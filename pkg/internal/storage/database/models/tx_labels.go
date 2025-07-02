package models

import (
	"gorm.io/gorm"
	"time"
)

type TransactionLabel struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt

	TransactionID uint
	LabelName     string
	LabelUserID   int
}
