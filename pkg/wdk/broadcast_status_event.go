package wdk

// BroadcastStatusEvent is a transaction lifecycle update pushed by the broadcaster
// (e.g. the Arcade SSE stream) rather than discovered by polling.
type BroadcastStatusEvent struct {
	// EventID is an opaque stream cursor (Arcade: nanosecond timestamp) used to resume the stream.
	EventID string
	TxID    string
	// Status is the broadcaster lifecycle status:
	// RECEIVED | SEEN_ON_NETWORK | SEEN_ON_MULTIPLE_NODES | MINED | IMMUTABLE | REJECTED.
	Status      string
	BlockHash   string
	BlockHeight uint32
	// MerklePath is the hex-encoded BUMP for mined transactions; may be empty.
	MerklePath   string
	ExtraInfo    string
	CompetingTxs []string
}
