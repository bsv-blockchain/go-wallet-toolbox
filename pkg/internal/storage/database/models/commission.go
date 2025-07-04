package models

import "gorm.io/gorm"

type Commission struct {
	gorm.Model

	UserID        int    `gorm:"index;not null"`
	TransactionID uint   `gorm:"index;not null"`
	Satoshis      uint64 `gorm:"not null"`
	KeyOffset     string `gorm:"type:string"`
	IsRedeemed    bool   `gorm:"not null;default:false"`
	LockingScript []byte `gorm:"not null"`
}
