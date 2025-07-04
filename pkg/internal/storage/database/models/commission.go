package models

import "gorm.io/gorm"

type Commission struct {
	gorm.Model

	UserID        int    `gorm:"not null;uniqueIndex:idx_commission_user_tx"`
	TransactionID uint   `gorm:"not null;uniqueIndex:idx_commission_user_tx"`
	Satoshis      uint64 `gorm:"not null"`
	KeyOffset     string `gorm:"type:string"`
	IsRedeemed    bool   `gorm:"not null;default:false"`
	LockingScript []byte `gorm:"not null"`
}
