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

	TxNotes []*TxHistoryNote
}

// TxHistoryNote represents a single transaction event note, combining general event metadata with a transaction ID.
type TxHistoryNote struct {
	wdk.HistoryNote
	TxID string
}

// KnownTxReadSpecification defines criteria for querying known transactions, including optional filtering by TxID.
type KnownTxReadSpecification struct {
	TxID  *string
	TxIDs []string

	IncludeHistoryNotes bool
	Status              *Comparable[wdk.ProvenTxReqStatus]
	Attempts            *Comparable[uint64]
	Notified            *Comparable[bool]
	BlockHeight         *Comparable[uint32]
	MerkleRoot          *Comparable[string]
	BlockHash           *Comparable[string]
}
