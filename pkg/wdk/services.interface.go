package wdk

import "github.com/bsv-blockchain/go-sdk/transaction"

// PostTxResultForTxID is the struct representing postTX result for particular TxID
type PostTxResultForTxID struct {
	TxID string
	// AlreadyKnown if true, the transaction was already known to this service. Usually treat as a success.
	// Potentially stop posting to additional transaction processors.
	AlreadyKnown bool
	// DoubleSpend is when service indicated this broadcast double spends at least one input
	// `competingTxs` may be an array of txids that were first seen spends of at least one input.
	DoubleSpend  bool
	BlockHash    *string
	BlockHeight  *int64
	MerklePath   *transaction.MerklePath
	CompetingTxs []string
	// TODO: Data type is object | string | PostTxResultForTxidError
	Data  any
	Notes []ReqHistoryNote
}

// PostBeefResult are properties on array items of result returned from postBeef method
type PostBeefResult struct {
	// Name is the name of the service to which the transaction was submitted for processing
	Name        string
	TxIDResults []PostTxResultForTxID
	// Data is service response object. Use service name and status to infer type of object.
	Data  any
	Notes []ReqHistoryNote
}

// Services defines an interface for handling 3rd party services
type Services interface {
	PostBeef(beef *transaction.Beef, txids []string) ([]*PostBeefResult, error)
}
