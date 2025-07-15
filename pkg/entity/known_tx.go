package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// KnownTx represents a record of a known transaction, its state and metadata relevant to synchronization and tracking.
// It aggregates wdk.ProvenTxReq and wdk.ProvenTx into a single entity for easier management and querying.
type KnownTx struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	TxID string

	Status   wdk.ProvenTxReqStatus
	Attempts uint64
	Notified bool

	RawTx     []byte
	InputBEEF []byte

	BlockHeight *uint32
	MerklePath  []byte
	MerkleRoot  *string
	BlockHash   *string
}

type KnownTxReadSpecification struct {
	TxID *string
	// TODO: Add more fields as needed for filtering or querying known transactions
}
