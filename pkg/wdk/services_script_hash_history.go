package wdk

// ScriptUsageStatusResult represents the result of a script usage status query
type ScriptUsageStatusResult struct {
	Name       string
	IsUsed     bool
	ScriptHash string
}

// ScriptHistoryItem represents a single transaction in script history
type ScriptHistoryItem struct {
	TxHash string
	Height *int
}

// ScriptHistoryResult represents the result of a script history query
type ScriptHistoryResult struct {
	Name       string
	ScriptHash string
	History    []ScriptHistoryItem
}
