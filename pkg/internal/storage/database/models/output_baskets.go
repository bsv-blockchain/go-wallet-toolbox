package models

import (
	"time"

	"gorm.io/gorm"
)

// OutputBasket is the database model of the output baskets
type OutputBasket struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Name   string `gorm:"primaryKey;type:varchar(300)"`
	UserID int    `gorm:"primaryKey"`

	NumberOfDesiredUTXOs    int64  `gorm:"not null;column:number_of_desired_utxos;default:32"`
	MinimumDesiredUTXOValue uint64 `gorm:"not null;default:1000"`
}
