package wdk

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// ListTransactionsArgs defines arguments for listing transactions with their status updates
type ListTransactionsArgs struct {
	// Limit is the maximum number of transactions to return
	Limit primitives.PositiveIntegerDefault10Max10000 `json:"limit,omitempty"`
	// Offset is the number of transactions to skip before returning results
	Offset primitives.PositiveInteger `json:"offset,omitempty"`
	// Status filters transactions by their update status
	Status *defs.TxUpdateStatus `json:"status,omitempty"`
	// TxID filters transactions by a specific transaction ID
	TxID *string `json:"txid,omitempty"`
	// Reference filters transactions by a specific reference string
	Reference *string `json:"reference,omitempty"`
}

// ListTransactionsResult defines the result of listing transactions
type ListTransactionsResult struct {
	// TotalTransactions is the total number of transactions matching the query
	TotalTransactions primitives.PositiveInteger `json:"totalTransactions"`
	// Transactions is the list of transaction status updates
	Transactions []defs.TransactionStatusUpdate `json:"transactions"`
}
