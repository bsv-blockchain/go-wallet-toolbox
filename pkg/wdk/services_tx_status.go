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

// Status represents the status of a transaction
type Status string

const (
	// TxStatusMined indicates the transaction has been mined
	TxStatusMined Status = "mined"
	// TxStatusUnconfirmed indicates the transaction is unconfirmed
	TxStatusUnconfirmed Status = "known"
	// TxStatusNotFound indicates the transaction was not found
	TxStatusNotFound Status = "unknown"
)

// String returns the string representation of the Status.
func (s Status) String() string {
	return string(s)
}
