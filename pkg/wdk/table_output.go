package wdk

import (
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

// TableOutput represents a service model based on the TS version.
type TableOutput struct {
	CreatedAt          time.Time                    `json:"created_at"`
	UpdatedAt          time.Time                    `json:"updated_at"`
	OutputID           uint                         `json:"outputId"`
	UserID             int                          `json:"userId"`
	TransactionID      uint                         `json:"transactionId"`
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

	// BasketName links the output to a specific basket. It is not a part of JSON RPC API.
	BasketName *string `json:"-"`

	// TODO: Link this field for sync/backup purposes.
	//BasketID           *int      `json:"basketId,omitempty"`
}
