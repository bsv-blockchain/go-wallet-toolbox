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
}
