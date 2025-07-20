package dto

// ScriptHistoryResponse represents the response from the script history endpoints
type ScriptHistoryResponse struct {
	ScriptHash string              `json:"scripthash,omitempty"`
	History    []ScriptHistoryItem `json:"history"`
	PgKey      string              `json:"pgkey,omitempty"`
	Error      string              `json:"error,omitempty"`
}

// ScriptHistoryItem represents a single entry in the script hash history response.
type ScriptHistoryItem struct {
	// TxID is the transaction ID associated with the script hash history entry.
	TxID string `json:"txid"`

	// Height is the block height at which the transaction was included (optional for unconfirmed)
	Height *int `json:"blockheight,omitempty"`
}
