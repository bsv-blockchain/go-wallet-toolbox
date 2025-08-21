package models

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

type Transaction struct {
	gorm.Model

	// Composite index:
	DeletedAt gorm.DeletedAt `gorm:"index:idx_transactions_user_id_status_deleted_at"`
	UserID    int            `gorm:"index:idx_transactions_user_id_status_deleted_at"`
	Status    wdk.TxStatus   `gorm:"index:idx_transactions_user_id_status_deleted_at"`

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
