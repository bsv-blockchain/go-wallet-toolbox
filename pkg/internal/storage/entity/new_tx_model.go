package entity

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/is"
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
	UTXOStatus        wdk.UTXOStatus

	Labels     []primitives.StringUnder300
	Commission *entity.Commission
}

// NewOutput represents an output of a new transaction.
type NewOutput struct {
	LockingScript      *primitives.HexString
	CustomInstructions *string
	Satoshis           satoshi.Value
	BasketName         *string
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
	Tags               []string
}

func (no *NewOutput) IsChangeOutputVout() bool {
	return is.Equal(no.Change, true) &&
		is.Equal(no.Purpose, wdk.ChangePurpose) &&
		is.Equal(no.ProvidedBy, wdk.ProvidedByStorage)
}

func (no *NewOutput) ToOutput(id uint, userID int, transactionID uint) (*entity.Output, error) {
	var lockingScript []byte
	if no.LockingScript != nil {
		var err error
		lockingScript, err = no.LockingScript.ToBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to convert locking script to bytes: %w", err)
		}
	}

	return &entity.Output{
		ID:                 id,
		UserID:             userID,
		TransactionID:      transactionID,
		Satoshis:           no.Satoshis.Int64(),
		Vout:               no.Vout,
		LockingScript:      lockingScript,
		CustomInstructions: no.CustomInstructions,
		DerivationPrefix:   no.DerivationPrefix,
		DerivationSuffix:   no.DerivationSuffix,
		BasketName:         no.BasketName,
		Spendable:          no.Spendable,
		Change:             no.Change,
		Description:        no.Description,
		ProvidedBy:         string(no.ProvidedBy),
		Purpose:            no.Purpose,
		Type:               string(no.Type),
		SenderIdentityKey:  no.SenderIdentityKey,
		Tags:               no.Tags,
	}, nil
}
