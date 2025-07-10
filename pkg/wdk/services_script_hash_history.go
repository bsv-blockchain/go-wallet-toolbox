package wdk

// ScriptHistoryItem represents a single transaction in script history
type ScriptHistoryItem struct {
	// TxHash is the transaction hash
	TxHash string
	// Height is the block height where the transaction was confirmed, nil if unconfirmed
	Height *int
}

// ScriptHistoryResult represents the result of a script history query
type ScriptHistoryResult struct {
	// Name is the name of the service providing this result
	Name string
	// ScriptHash is the script hash for which the history was retrieved
	ScriptHash string
	// History contains the list of script history items
	History []ScriptHistoryItem
}
