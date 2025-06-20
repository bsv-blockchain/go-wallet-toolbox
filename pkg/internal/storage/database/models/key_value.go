package models

import "gorm.io/gorm"

type KeyValue struct {
	gorm.Model

	Key   string `gorm:"primaryKey"`
	Value []byte
}
