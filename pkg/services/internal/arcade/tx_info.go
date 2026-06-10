package arcade

// TxStatus is the lifecycle status of a transaction reported by Arcade.
type TxStatus string

// List of statuses reported by Arcade: https://github.com/bsv-blockchain/arcade
const (
	// StatusReceived means the transaction has been received by Arcade.
	StatusReceived TxStatus = "RECEIVED"
	// StatusSeenOnNetwork means the transaction has been seen on the Bitcoin network.
	StatusSeenOnNetwork TxStatus = "SEEN_ON_NETWORK"
	// StatusSeenOnMultipleNodes means the transaction has been seen on multiple Bitcoin nodes.
	StatusSeenOnMultipleNodes TxStatus = "SEEN_ON_MULTIPLE_NODES"
	// StatusMined means the transaction has been mined into a block.
	StatusMined TxStatus = "MINED"
	// StatusImmutable means the transaction is buried deep enough to be considered immutable.
	StatusImmutable TxStatus = "IMMUTABLE"
	// StatusRejected means the transaction has been rejected by the Bitcoin network.
	StatusRejected TxStatus = "REJECTED"
)

// TXInfo is the transaction information returned by Arcade
// (broadcast response, query response and SSE status events share this shape).
type TXInfo struct {
	TxID         string   `json:"txid"`
	TxStatus     TxStatus `json:"txStatus"`
	Timestamp    string   `json:"timestamp"`
	BlockHash    string   `json:"blockHash"`
	BlockHeight  uint32   `json:"blockHeight"`
	MerklePath   string   `json:"merklePath"`
	ExtraInfo    string   `json:"extraInfo"`
	CompetingTxs []string `json:"competingTxs"`
}
