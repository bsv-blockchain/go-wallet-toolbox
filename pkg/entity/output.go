package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Output represents a transaction output in the wallet.
type Output struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time

	UserID        int
	TransactionID uint
	SpentBy       *uint
	Satoshis      int64

	TxID     *string //NOTE: TxID can be nil if the owning transaction is not yet processed.
	TxStatus wdk.TxStatus
	Vout     uint32

	LockingScript      []byte
	CustomInstructions *string

	DerivationPrefix *string
	DerivationSuffix *string

	BasketName *string

	Spendable bool
	Change    bool

	Description string
	ProvidedBy  string
	Purpose     string
	Type        string

	SenderIdentityKey *string

	Tags []string

	UserUTXO *UserUTXO
}

// ToWDK converts the Output entity to its WDK representation.
func (o *Output) ToWDK() *wdk.TableOutput {
	return &wdk.TableOutput{
		CreatedAt:          o.CreatedAt,
		UpdatedAt:          o.UpdatedAt,
		OutputID:           o.ID,
		UserID:             o.UserID,
		TransactionID:      o.TransactionID,
		SpentBy:            o.SpentBy,
		Spendable:          o.Spendable,
		Change:             o.Change,
		OutputDescription:  o.Description,
		Vout:               o.Vout,
		Satoshis:           o.Satoshis,
		ProvidedBy:         o.ProvidedBy,
		Purpose:            o.Purpose,
		Type:               o.Type,
		TxID:               o.TxID,
		DerivationPrefix:   o.DerivationPrefix,
		DerivationSuffix:   o.DerivationSuffix,
		CustomInstructions: o.CustomInstructions,
		LockingScript:      o.LockingScript,
		SenderIdentityKey:  o.SenderIdentityKey,
	}
}

// OutputReadSpecification defines filter criteria for querying outputs.
type OutputReadSpecification struct {
	ID            *uint
	UserID        *Comparable[int]
	TransactionID *Comparable[uint]
	SpentBy       *Comparable[uint]
	BasketName    *Comparable[string]
	Spendable     *Comparable[bool]
	Change        *Comparable[bool]
	TxStatus      *Comparable[wdk.TxStatus]
	Satoshis      *Comparable[int64]
	TxID          *Comparable[string]
	Vout          *Comparable[uint32]
	Tags          *ComparableSet[string]
}

// OutputUpdateSpecification defines fields that can be updated.
type OutputUpdateSpecification struct {
	ID            uint
	Spendable     *bool
	Description   *string
	LockingScript *[]byte
	CustomInstr   *string
}
