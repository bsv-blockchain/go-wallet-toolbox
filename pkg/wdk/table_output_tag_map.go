package wdk

import "time"

// TableOutputTagMap represents the mapping between an output tag and a transaction in a table.
type TableOutputTagMap struct {
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	OutputTagID   int       `json:"outputTagId"`
	TransactionID uint      `json:"transactionId"`
	IsDeleted     bool      `json:"isDeleted"`
}
