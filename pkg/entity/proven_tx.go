package entity

import "time"

// ProvenTx represents a confirmed blockchain transaction in the system.
type ProvenTx struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time

	TxID       string
	Height     uint32
	Index      uint32
	MerklePath []byte
	RawTx      []byte
	BlockHash  string
	MerkleRoot string
}

// ProvenTxReadSpecification defines criteria for querying proven transactions.
type ProvenTxReadSpecification struct {
	ID    *uint
	TxID  *string
	TxIDs []string
}
