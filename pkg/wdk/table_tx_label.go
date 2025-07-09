package wdk

import "time"

// TableTxLabel represents a transaction label record in the table with metadata such as timestamps and user association.
type TableTxLabel struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TxLabelID uint      `json:"txLabelId"`
	UserID    int       `json:"userId"`
	Label     string    `json:"label"`
	IsDeleted bool      `json:"isDeleted"`
}
