package results

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// PostBEEF is the success result of the single service PostBEEF method.
type PostBEEF struct {
	Notes
	TxIDResults []PostTxID
}

// ResultStatus is the status of the result which can be either success or error.
type ResultStatus string

const (
	// ResultStatusSuccess indicates that the result was a success.
	ResultStatusSuccess ResultStatus = "success"
	// ResultStatusError indicates that the result was an error.
	ResultStatusError ResultStatus = "error"
)

// PostTxID is the struct representing postTX result for particular TxID
type PostTxID struct {
	Result ResultStatus
	TxID   string
	// AlreadyKnown if true, the transaction was already known to this service. Usually treat as a success.
	// Potentially stop posting to additional transaction processors.
	AlreadyKnown bool
	// DoubleSpend is when service indicated this broadcast double spends at least one input
	// `competingTxs` may be an array of txids that were first seen spends of at least one input.
	DoubleSpend  bool
	BlockHash    string
	BlockHeight  int64
	MerklePath   *transaction.MerklePath
	CompetingTxs []string
	// TODO: consider making it a string
	Data any
	Notes
	Error error
}
