package entity

import (
	"time"
)

// TxNote represents a transaction event with metadata including time, user information, and event attributes.
// It's an equivalent to the HistoryNote in wdk.ProvenTxReq
type TxNote struct {
	When time.Time

	TxID   string
	UserID *int

	What       string
	Attributes map[string]any
}
