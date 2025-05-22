package entity

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
)

// NewTx represents all the information necessary to store a transaction with additional information like labels, tags, inputs, and outputs.
// This meant to be used for createAction
type NewTx struct {
	UserID int

	Version     uint32
	LockTime    uint32
	Status      wdk.TxStatus
	Reference   string
	Satoshis    int64
	IsOutgoing  bool
	InputBeef   []byte
	Description string

	TxID *string

	ReservedOutputIDs []uint
	Outputs           []*NewOutput

	Labels []primitives.StringUnder300
}

// NewOutput represents an output of a new transaction.
type NewOutput struct {
	LockingScript      *primitives.HexString
	CustomInstructions *string
	Satoshis           satoshi.Value
	Basket             *string
	Spendable          bool
	Change             bool
	ProvidedBy         wdk.ProvidedBy
	Purpose            string
	Type               wdk.OutputType
	DerivationPrefix   *string
	DerivationSuffix   *string
	Description        string
	Vout               uint32
	SenderIdentityKey  *string
}
