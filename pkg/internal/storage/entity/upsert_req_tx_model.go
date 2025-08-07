package entity

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type UpsertKnownTx struct {
	InputBeef     []byte
	RawTx         []byte
	TxID          string
	Status        wdk.ProvenTxReqStatus
	SkipForStatus *wdk.ProvenTxReqStatus
	MerklePath    *UpsertKnownTxWithMerklePath
}

type UpsertKnownTxWithMerklePath struct {
	BlockHeight uint32
	MerklePath  []byte
	MerkleRoot  string
	BlockHash   string
}
