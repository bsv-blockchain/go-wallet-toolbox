package entity

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type KnownTxForStatusSync struct {
	TxID     string
	Attempts uint64
	Status   wdk.ProvenTxReqStatus
	Batch    *string
}

type KnownTxAsMined struct {
	TxID        string
	BlockHeight uint32
	MerklePath  []byte
	MerkleRoot  string
	BlockHash   string
	Notes       []history.Builder
}
