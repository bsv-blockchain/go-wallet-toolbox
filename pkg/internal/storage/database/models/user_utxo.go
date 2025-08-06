package models

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// UserUTXO is a table holding user's Unspent Transaction Outputs (UTXOs).
type UserUTXO struct {
	UserID   int     `gorm:"primaryKey"`
	OutputID uint    `gorm:"primaryKey"`
	Output   *Output `gorm:"foreignKey:OutputID"`

	UTXOStatus wdk.UTXOStatus `gorm:"index:idx_utxo_status"`

	BasketName string        `gorm:"not null,index"`
	Basket     *OutputBasket `gorm:"foreignKey:UserID,BasketName;references:UserID,Name"`

	Satoshis uint64
	// EstimatedInputSize is the estimated size increase when adding and unlocking this UTXO to a transaction.
	EstimatedInputSize uint64
	CreatedAt          time.Time

	ReservedByID *uint
	ReservedBy   *Transaction
}
