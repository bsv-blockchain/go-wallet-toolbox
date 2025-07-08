package models

import (
	"time"

	"gorm.io/gorm"
)

type Label struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Name   string `gorm:"primarykey"`
	UserID int    `gorm:"primarykey"`

	Transactions []*Transaction `gorm:"many2many:transaction_labels;"`
}
