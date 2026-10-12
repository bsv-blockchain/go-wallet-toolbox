package wdk

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// TxSynchronizedStatus represents the synchronization status of a transaction.
type TxSynchronizedStatus struct {
	TxID      string
	Reference string
	Status    ProvenTxReqStatus

	MerkleRoot  string
	MerklePath  *transaction.MerklePath
	BlockHeight uint32
	BlockHash   string
	Labels      []string
}

// ReproveHeaderResult reports what happened to the transactions proven in a block that a
// reorg orphaned (TypeScript reproveHeader taxonomy).
type ReproveHeaderResult struct {
	// Updated holds transactions whose proof was moved to a block on the active chain.
	Updated []TxSynchronizedStatus
	// Unchanged lists txIDs whose only available proof still points at the orphaned block.
	Unchanged []string
	// Unavailable lists txIDs with no valid replacement proof yet.
	Unavailable []string
}
