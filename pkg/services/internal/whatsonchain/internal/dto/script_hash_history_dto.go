package dto

// ScriptHashHistoryResponse represents the response from the script history endpoints
type ScriptHashHistoryResponse struct {
	// ScriptHash is the script hash for which the history is being retrieved (not always present)
	ScriptHash string `json:"script,omitempty"`

	// Result contains the history of transactions associated with the script hash.
	Result []ScriptHashHistoryItem `json:"result"`

	// Error is an error message if the request failed, otherwise it is empty.
	Error string `json:"error,omitempty"`
}

// ScriptHashHistoryItem represents a single entry in the script hash history response.
type ScriptHashHistoryItem struct {
	// TxID is the transaction ID associated with the script hash history entry.
	TxID string `json:"tx_hash"`

	// Height is the block height at which the transaction was included (optional for unconfirmed)
	Height *int `json:"height,omitempty"`
}
