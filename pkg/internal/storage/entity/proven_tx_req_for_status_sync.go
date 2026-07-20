package entity

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/history"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type ProvenTxReqForStatusSync struct {
	TxID                string
	Attempts            uint64
	RebroadcastAttempts uint64
	Status              wdk.ProvenTxReqStatus
	WasBroadcast        bool
	Batch               *string
}

type ProvenTxAsMined struct {
	TxID        string
	BlockHeight uint32
	MerklePath  []byte
	MerkleRoot  string
	BlockHash   string
	Notes       []history.Builder
}
