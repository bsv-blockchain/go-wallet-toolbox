package dto

// ScriptHashUnspentItem represents an unspent output
type ScriptHashUnspentItem struct {
	Height int64  `json:"height"`
	TxPos  uint32 `json:"tx_pos"`
	TxHash string `json:"tx_hash"`
	Value  uint64 `json:"value"`
}

// ScriptHashUnspentResponse represents the full unspent response
type ScriptHashUnspentResponse struct {
	Script string                  `json:"script"`
	Result []ScriptHashUnspentItem `json:"result"`
	Error  string                  `json:"error"`
}
