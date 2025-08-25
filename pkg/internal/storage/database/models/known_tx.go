package models

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type KnownTx struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	TxID string `gorm:"type:varchar(64);primaryKey"`

	Status   wdk.ProvenTxReqStatus `gorm:"default:unknown"`
	Attempts uint64
	Notified bool
	Batch    *string `gorm:"index"`

	RawTx     []byte
	InputBeef []byte

	BlockHeight *uint32
	MerklePath  []byte
	MerkleRoot  *string
	BlockHash   *string

	TxNotes []*TxNote `gorm:"foreignKey:TxID;references:TxID"`
}

// HasMerklePath returns true if the MerklePath field contains data, indicating the presence of a Merkle proof.
func (p *KnownTx) HasMerklePath() bool {
	return len(p.MerklePath) > 0
}
