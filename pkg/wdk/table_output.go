package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TableOutput represents a service model based on the TS version.
type TableOutput struct {
	CreatedAt          time.Time                    `json:"created_at"`
	UpdatedAt          time.Time                    `json:"updated_at"`
	OutputID           uint                         `json:"outputId"`
	UserID             int                          `json:"userId"`
	TransactionID      uint                         `json:"transactionId"`
	SpentBy            *uint                        `json:"spentBy,omitempty"`
	Spendable          bool                         `json:"spendable"`
	Change             bool                         `json:"change"`
	OutputDescription  string                       `json:"outputDescription"`
	Vout               uint32                       `json:"vout"`
	Satoshis           int64                        `json:"satoshis"`
	ProvidedBy         string                       `json:"providedBy"`
	Purpose            string                       `json:"purpose"`
	Type               string                       `json:"type"`
	TxID               *string                      `json:"txid,omitempty"`
	DerivationPrefix   *string                      `json:"derivationPrefix,omitempty"`
	DerivationSuffix   *string                      `json:"derivationSuffix,omitempty"`
	CustomInstructions *string                      `json:"customInstructions,omitempty"`
	LockingScript      primitives.ExplicitByteArray `json:"lockingScript,omitempty"`
	SenderIdentityKey  *string                      `json:"senderIdentityKey,omitempty"`
	BasketID           *int                         `json:"basketId,omitempty"`
}

// TableOutputs is a list of TableOutput items.
type TableOutputs = []TableOutput

// FindOutputsArgs represents the arguments for finding outputs.
type FindOutputsArgs struct {
	UserID        *int    `json:"userId,omitempty"`
	OutputID      *uint   `json:"outputId,omitempty"`
	Satoshis      *int64  `json:"satoshis,omitempty"`
	TransactionID *uint   `json:"transactionId,omitempty"`
	TxID          *string `json:"txid,omitempty"`
	Vout          *uint32 `json:"vout,omitempty"`
	Change        *bool   `json:"change,omitempty"`
	Spendable     *bool   `json:"spendable,omitempty"`

	TxStatus []TxStatus `json:"txStatus,omitempty"`
}
