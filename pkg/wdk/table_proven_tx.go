package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TableProvenTx represents a transaction that has been proven and stored in the database.
type TableProvenTx struct {
	CreatedAt  time.Time                    `json:"created_at"`
	UpdatedAt  time.Time                    `json:"updated_at"`
	ProvenTxID int                          `json:"provenTxId"`
	TxID       string                       `json:"txid"`
	Height     uint32                       `json:"height"`
	Index      int                          `json:"index"`
	MerklePath primitives.ExplicitByteArray `json:"merklePath"`
	RawTx      primitives.ExplicitByteArray `json:"rawTx"`
	BlockHash  string                       `json:"blockHash"`
	MerkleRoot string                       `json:"merkleRoot"`
}
