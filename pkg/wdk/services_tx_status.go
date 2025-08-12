package wdk

// TxStatusDetail holds the status of a single txid
type TxStatusDetail struct {
	TxID   string
	Depth  *int
	Status string
}

// GetStatusForTxIDsResult represents result of a GetStatusForTxIDs query
type GetStatusForTxIDsResult struct {
	Name    string
	Status  GetStatusResult
	Results []TxStatusDetail
}

// GetStatusResult represents the status of a GetStatusForTxids query
type GetStatusResult string

const (
	// GetStatusSuccess indicates the query was successful
	GetStatusSuccess GetStatusResult = "success"
)

// ResultStatusForTxID represents the status of a transaction
type ResultStatusForTxID string

const (
	// ResultStatusForTxIDMined indicates the transaction has been mined
	ResultStatusForTxIDMined ResultStatusForTxID = "mined"
	// ResultStatusForTxIDKnown indicates the transaction is unconfirmed
	ResultStatusForTxIDKnown ResultStatusForTxID = "known"
	// ResultStatusForTxIDNotFound indicates the transaction was not found
	ResultStatusForTxIDNotFound ResultStatusForTxID = "unknown"
)

// String returns the string representation of the Status.
func (s ResultStatusForTxID) String() string {
	return string(s)
}
