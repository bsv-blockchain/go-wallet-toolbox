package entity

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"time"
)

type ProvenTxReq struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	TxID string

	Status   wdk.ProvenTxReqStatus
	Attempts uint64
	Notified bool

	RawTx     []byte
	InputBEEF []byte

	// TODO: History field

	BlockHeight *uint32
	MerklePath  []byte
	MerkleRoot  *string
	BlockHash   *string
}
