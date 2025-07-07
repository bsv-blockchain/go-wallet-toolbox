package wdk

import "time"

// TableTxLabelMap represents a mapping between transaction labels and transactions in the table with metadata such as timestamps and user association.
type TableTxLabelMap struct {
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	TxLabelID     uint      `json:"txLabelId"`
	TransactionID uint      `json:"transactionId"`
	IsDeleted     bool      `json:"isDeleted"`
}
