package wdk

// RawTxResult is result from RawTx method
type RawTxResult struct {
	// TxID is a transaction hash or rawTx
	TxID string
	// Name is the name of the service returning the rawTx or nil if no rawTx
	Name string
	// RawTx are multiple proofs that may be returned when a transaction also appears in
	// one or more orphaned blocks
	RawTx []byte
}
