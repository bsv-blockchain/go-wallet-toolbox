package models

import (
	"time"
)

type KeyValue struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	Key   string `gorm:"primaryKey"`
	Value []byte
}
