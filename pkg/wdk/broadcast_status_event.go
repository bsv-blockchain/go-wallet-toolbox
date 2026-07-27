package wdk

// BroadcastStatusEvent is a transaction lifecycle update pushed by the broadcaster
// (e.g. the Arcade SSE stream) rather than discovered by polling.
type BroadcastStatusEvent struct {
	// EventID is an opaque stream cursor (Arcade: nanosecond timestamp) used to resume the stream.
	EventID string
	TxID    string
	// Status is the broadcaster lifecycle status. Arcade (models.Status) values include:
	// RECEIVED | SENT_TO_NETWORK | ACCEPTED_BY_NETWORK | SEEN_ON_NETWORK |
	// SEEN_MULTIPLE_NODES | STUMP_PROCESSING | MINED | IMMUTABLE | REJECTED |
	// DOUBLE_SPEND_ATTEMPTED | PENDING_RETRY (and legacy SEEN_ON_MULTIPLE_NODES).
	Status      string
	BlockHash   string
	BlockHeight uint32
	// MerklePath is the hex-encoded BRC-74 BUMP for MINED/IMMUTABLE frames when
	// Arcade includes it on the SSE event; empty on pre-mined frames and when
	// catchup omits the path (client falls back to GET /tx or polling).
	MerklePath   string
	ExtraInfo    string
	CompetingTxs []string
}
