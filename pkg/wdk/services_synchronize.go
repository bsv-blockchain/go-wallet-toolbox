package wdk

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
)

type TxSynchronizedStatus struct {
	TxID   string
	Status ProvenTxReqStatus

	MerkleRoot  string
	MerklePath  *transaction.MerklePath
	BlockHeight uint32
	BlockHash   string
}
