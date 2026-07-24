package arcade

// TxStatus is the lifecycle status of a transaction reported by Arcade.
// Values match github.com/bsv-blockchain/arcade models.Status (main branch).
type TxStatus string

// Statuses reported by Arcade SSE / webhooks / GET /tx.
// Source of truth: arcade/models/transaction.go AllStatuses() and docs/sse.md.
//
// Lifecycle (typical happy path):
//
//	RECEIVED → SENT_TO_NETWORK → ACCEPTED_BY_NETWORK → SEEN_ON_NETWORK
//	  → SEEN_MULTIPLE_NODES → STUMP_PROCESSING → MINED → IMMUTABLE
//
// MINED / IMMUTABLE frames additionally carry blockHash, blockHeight, and
// merklePath (BRC-74 BUMP hex) so push-only clients need not poll for proofs.
const (
	// StatusUnknown indicates the transaction status is unknown.
	StatusUnknown TxStatus = "UNKNOWN"
	// StatusReceived means the transaction has been received by Arcade.
	StatusReceived TxStatus = "RECEIVED"
	// StatusSentToNetwork means Arcade submitted the transaction to Teranode.
	StatusSentToNetwork TxStatus = "SENT_TO_NETWORK"
	// StatusAcceptedByNetwork means a connected node accepted the transaction.
	StatusAcceptedByNetwork TxStatus = "ACCEPTED_BY_NETWORK"
	// StatusSeenOnNetwork means the transaction was seen on the Bitcoin network.
	StatusSeenOnNetwork TxStatus = "SEEN_ON_NETWORK"
	// StatusSeenMultipleNodes means the transaction was seen by multiple miners.
	// This is the canonical Arcade name (not SEEN_ON_MULTIPLE_NODES).
	StatusSeenMultipleNodes TxStatus = "SEEN_MULTIPLE_NODES"
	// StatusSeenOnMultipleNodes is a legacy / docs-only alias still accepted by
	// storage ProcessExternalTxStatusUpdate. Prefer StatusSeenMultipleNodes.
	StatusSeenOnMultipleNodes TxStatus = "SEEN_ON_MULTIPLE_NODES"
	// StatusDoubleSpendAttempted means a double-spend was reported.
	StatusDoubleSpendAttempted TxStatus = "DOUBLE_SPEND_ATTEMPTED"
	// StatusRejected means the transaction has been rejected by the network.
	StatusRejected TxStatus = "REJECTED"
	// StatusPendingRetry means broadcast failed with a retryable error.
	StatusPendingRetry TxStatus = "PENDING_RETRY"
	// StatusStumpProcessing means a STUMP was received and a BUMP is being built.
	StatusStumpProcessing TxStatus = "STUMP_PROCESSING"
	// StatusMined means the transaction has been mined into a block.
	// SSE frames at this status include merklePath (BUMP hex) when available.
	StatusMined TxStatus = "MINED"
	// StatusImmutable means the transaction is buried deep enough to be final.
	// Same proof fields as MINED when present on the SSE frame.
	StatusImmutable TxStatus = "IMMUTABLE"

	// ARC-only intermediates that may appear if a host mixes ARC status names
	// into an Arcade-compatible stream (not in arcade.AllStatuses()).
	StatusAnnouncedToNetwork TxStatus = "ANNOUNCED_TO_NETWORK"
	StatusStored             TxStatus = "STORED"
)

// AllStatuses returns Arcade lifecycle statuses in roughly forward order,
// mirroring arcade/models.Status AllStatuses() (without ARC-only extras).
func AllStatuses() []TxStatus {
	return []TxStatus{
		StatusUnknown,
		StatusReceived,
		StatusSentToNetwork,
		StatusAcceptedByNetwork,
		StatusSeenOnNetwork,
		StatusSeenMultipleNodes,
		StatusDoubleSpendAttempted,
		StatusRejected,
		StatusPendingRetry,
		StatusStumpProcessing,
		StatusMined,
		StatusImmutable,
	}
}

// TXInfo is the transaction information returned by Arcade
// (broadcast response, query response and SSE status events share this shape).
//
// Per docs/sse.md, every status frame has txid, txStatus, timestamp. MINED and
// IMMUTABLE frames additionally carry blockHash, blockHeight, and merklePath
// (BRC-74 BUMP hex). Catchup replay may omit merklePath under load — clients
// should fall back to GET /tx/{txid}.
type TXInfo struct {
	TxID         string   `json:"txid"`
	TxStatus     TxStatus `json:"txStatus"`
	Timestamp    string   `json:"timestamp"`
	BlockHash    string   `json:"blockHash"`
	BlockHeight  uint32   `json:"blockHeight"`
	MerklePath   string   `json:"merklePath"` // BRC-74 BUMP hex when present
	ExtraInfo    string   `json:"extraInfo"`
	CompetingTxs []string `json:"competingTxs"`
}
