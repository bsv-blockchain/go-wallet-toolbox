package wdk

import (
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

// TableTransaction is a struct that represents transaction details
type TableTransaction struct {
	CreatedAt     time.Time                    `json:"created_at"`
	UpdatedAt     time.Time                    `json:"updated_at"`
	TransactionID uint                         `json:"transactionId"`
	UserID        int                          `json:"userId"`
	ProvenTxID    *string                      `json:"proveTxId"`
	Status        TxStatus                     `json:"status"`
	Reference     primitives.Base64String      `json:"reference"`
	IsOutgoing    bool                         `json:"isOutgoing"`
	Satoshis      int64                        `json:"satoshis"`
	Description   string                       `json:"description"`
	Version       *uint32                      `json:"version"`
	LockTime      *uint32                      `json:"lockTime"`
	TxID          *string                      `json:"txid"`
	InputBEEF     primitives.ExplicitByteArray `json:"inputBEEF"`
}
