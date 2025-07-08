package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type KnownTx struct {
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
