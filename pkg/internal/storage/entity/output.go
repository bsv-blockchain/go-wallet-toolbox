package entity

import (
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

	TxID *string //NOTE: TxID can be nil if the owning transaction is not yet processed.
	Vout uint32

	LockingScript      []byte
	CustomInstructions *string

	DerivationPrefix *string
	DerivationSuffix *string

	BasketName *string
	Basket     *OutputBasket

	Spendable bool
	Change    bool

	Description string
	ProvidedBy  string
	Purpose     string
	Type        string

	SenderIdentityKey *string

	Tags []string
}
