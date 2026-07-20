package models

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type KnownTx struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	TxID string `gorm:"type:varchar(64);primaryKey"`

	Status              wdk.ProvenTxReqStatus `gorm:"default:unknown"`
	Attempts            uint64
	RebroadcastAttempts uint64 `gorm:"column:rebroadcast_attempts;default:0"`
	Notified            bool
	Batch               *string `gorm:"index"`
	// Notify is an opaque JSON blob used by the TypeScript wallet to track
	// which transactions to notify when proofs arrive. Stored and returned
	// unchanged for sync round-trip compatibility.
	Notify string `gorm:"type:text;default:'{}'"`

	WasBroadcast bool

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
