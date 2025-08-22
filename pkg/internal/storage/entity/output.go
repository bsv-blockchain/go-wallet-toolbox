package entity

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"time"
)

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
