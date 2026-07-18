package models

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// UserUTXO is a table holding user's Unspent Transaction Outputs (UTXOs).
//
// idx_user_utxos_selection is a composite index over (user_id, basket_name,
// reserved_by_id, utxo_status, satoshis) supporting UTXO selection queries that
// filter/sort on this exact column set. ReservedByID also carries a plain,
// single-column index so lookups by reservation alone don't need the composite.
type UserUTXO struct {
	UserID   int     `gorm:"primaryKey;index:idx_user_utxos_selection,priority:1"`
	OutputID uint    `gorm:"primaryKey"`
	Output   *Output `gorm:"foreignKey:OutputID"`

	UTXOStatus wdk.UTXOStatus `gorm:"index:idx_utxo_status;index:idx_user_utxos_selection,priority:4"`

	BasketName string        `gorm:"not null;index;index:idx_user_utxos_selection,priority:2"`
	Basket     *OutputBasket `gorm:"foreignKey:UserID,BasketName;references:UserID,Name"`

	Satoshis uint64 `gorm:"index:idx_user_utxos_selection,priority:5"`
	// EstimatedInputSize is the estimated size increase when adding and unlocking this UTXO to a transaction.
	EstimatedInputSize uint64
	CreatedAt          time.Time

	ReservedByID *uint `gorm:"index;index:idx_user_utxos_selection,priority:3"`
	ReservedBy   *Transaction
}
