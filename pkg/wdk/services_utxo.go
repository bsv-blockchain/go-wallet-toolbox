package wdk

// UtxoDetail describes each UTXO
type UtxoDetail struct {
	TxID     string
	Index    uint32
	Height   int64
	Satoshis uint64
}

// UtxoStatusResult encapsulates UTXO query results
type UtxoStatusResult struct {
	Name    string
	Details []UtxoDetail
	IsUtxo  bool
}
