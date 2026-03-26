package models

import (
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Transaction struct {
	gorm.Model

	UserID      int
	Status      wdk.TxStatus
	Reference   string `gorm:"uniqueIndex"`
	IsOutgoing  bool
	Satoshis    int64
	Description string `gorm:"type:string"`
	Version     uint32
	LockTime    uint32
	TxID        *string `gorm:"index"`
	InputBeef   []byte

	Outputs       []*Output   `gorm:"foreignKey:TransactionID"`
	Inputs        []*Output   `gorm:"foreignKey:SpentBy"`
	Labels        []*Label    `gorm:"many2many:transaction_labels;"`
	ReservedUtxos []*UserUTXO `gorm:"foreignKey:ReservedByID"`
	Commission    *Commission `gorm:"foreignKey:TransactionID"`
}
