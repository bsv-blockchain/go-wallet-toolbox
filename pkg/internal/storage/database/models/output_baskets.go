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

	BasketID                int    `gorm:"primaryKey;not null"`
	Name                    string `gorm:"type:varchar(300);not null;uniqueIndex:idx_name_user_id"`
	NumberOfDesiredUTXOs    int64  `gorm:"not null;column:number_of_desired_utxos;default:32"`
	MinimumDesiredUTXOValue uint64 `gorm:"not null;default:1000"`

	UserID int `gorm:"uniqueIndex:idx_name_user_id"`
}
