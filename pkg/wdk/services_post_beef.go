package wdk

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/seq"
)

// PostBeefResult is a list of results from the PostBEEF method of all services.
type PostBeefResult []*PostBEEFServiceResult

// Success checks if one of the results is a success.
func (it PostBeefResult) Success() bool {
	return seq.Exists(seq.FromSlice(it), func(it *PostBEEFServiceResult) bool {
		return it.Success()
	})
}

// Aggregated gets results from all services and aggregates them by txid, calculating status and counts.
func (it PostBeefResult) Aggregated(txids []string) AggregatedPostBEEF {
	return newAggregatedPostBEEF(it, txids)
}

// ServiceErrors returns a map containing service names and their corresponding errors for failed PostBEEF results.
func (it PostBeefResult) ServiceErrors() map[string]error {
	errs := make(map[string]error)
	for _, result := range it {
		if result.Error != nil {
			errs[result.Name] = result.Error
		}
	}
	return errs
}

// PostBEEFServiceResult is the result of the PostBEEF method of a single service.
// It contains the name of the service that produced the result and the result itself.
// The result could be either a success or an error.
type PostBEEFServiceResult struct {
	Name             string
	PostedBEEFResult *PostedBEEF
	Error            error
}

// Success checks if the result is a success.
func (it *PostBEEFServiceResult) Success() bool {
	return it.PostedBEEFResult != nil && it.Error == nil
}

// PostedBEEF is the success result of the single service PostedBEEF method.
type PostedBEEF struct {
	TxIDResults []PostedTxID
}

// PostedTxIDResultStatus is the status of the result which can be either success or error.
type PostedTxIDResultStatus string

const (
	// PostedTxIDResultSuccess indicates that the result was a success.
	PostedTxIDResultSuccess PostedTxIDResultStatus = "success"
	// PostedTxIDResultError indicates that the result was an error.
	PostedTxIDResultError PostedTxIDResultStatus = "error"
	// PostedTxIDResultAlreadyKnown indicates that the transaction was already known to this service.
	PostedTxIDResultAlreadyKnown PostedTxIDResultStatus = "already_known"
	// PostedTxIDResultDoubleSpend indicates that the transaction double spends at least one input.
	PostedTxIDResultDoubleSpend PostedTxIDResultStatus = "double_spend"
	// PostedTxIDResultMissingInputs indicates that the transaction is missing inputs, possibly due to a double spend.
	PostedTxIDResultMissingInputs PostedTxIDResultStatus = "missing_inputs"
)

// PostedTxID is the struct representing postTX result for particular TxID
type PostedTxID struct {
	Result PostedTxIDResultStatus
	TxID   string
	// AlreadyKnown if true, the transaction was already known to this service. Usually treat as a success.
	// Potentially stop posting to additional transaction processors.
	AlreadyKnown bool
	// DoubleSpend is when service indicated this broadcast double spends at least one input
	DoubleSpend bool
	BlockHash   string
	BlockHeight uint32
	MerklePath  *transaction.MerklePath

	// CompetingTxs may be an array of txids that were first seen spends of at least one input.
	CompetingTxs []string

	Notes HistoryNotes

	Data  string
	Error error
}
