package models

import (
	"github.com/go-softwarelab/common/pkg/must"
	"time"

	"gorm.io/gorm"
)

// OutputBasket is the database model of the output baskets
type OutputBasket struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Num       int

	Name   string `gorm:"primaryKey;type:varchar(300)"`
	UserID int    `gorm:"primaryKey"`

	NumberOfDesiredUTXOs    int64  `gorm:"not null;column:number_of_desired_utxos;default:32"`
	MinimumDesiredUTXOValue uint64 `gorm:"not null;default:1000"`
}

func (ob *OutputBasket) BeforeCreate(tx *gorm.DB) (err error) {
	var count int64

	if err = tx.Model(&OutputBasket{}).Unscoped().Count(&count).Error; err != nil {
		return err
	}

	ob.Num = must.ConvertToInt(count)

	return nil
}
