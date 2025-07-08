package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TableProvenTxReq represents a persisted request for a proven transaction, including status and metadata for processing.
type TableProvenTxReq struct {
	CreatedAt     time.Time                    `json:"created_at"`
	UpdatedAt     time.Time                    `json:"updated_at"`
	ProvenTxReqID int                          `json:"provenTxReqId"`
	ProvenTxID    *int                         `json:"provenTxId,omitempty"`
	Status        ProvenTxReqStatus            `json:"status"`
	Attempts      uint64                       `json:"attempts"`
	Notified      bool                         `json:"notified"`
	TxID          string                       `json:"txid"`
	Batch         *string                      `json:"batch,omitempty"`
	History       string                       `json:"history"`
	Notify        string                       `json:"notify"`
	RawTx         primitives.ExplicitByteArray `json:"rawTx"`
	InputBEEF     primitives.ExplicitByteArray `json:"inputBEEF,omitempty"`
}
