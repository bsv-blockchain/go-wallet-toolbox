package models

import (
	"gorm.io/gorm"
)

type Output struct {
	gorm.Model

	// UserID is the second column of idx_bsv_outputs_transaction_id_user_id (see TransactionID).
	UserID int `gorm:"index;index:idx_bsv_outputs_transaction_id_user_id,priority:2"`
	// TransactionID keeps its single-column index and also leads
	// idx_bsv_outputs_transaction_id_user_id. SQLite without ANALYZE has no statistics, so it
	// prefers an index it believes is more selective: the composite makes the per-transaction
	// change-output lookup (transaction_id and user_id) use it instead of scanning every output of
	// the user, and the single-column index keeps transaction_id-only lookups from losing to the
	// low-selectivity spendable index.
	TransactionID uint   `gorm:"index;index:idx_bsv_outputs_transaction_id_user_id,priority:1"`
	SpentBy       *uint  `gorm:"index"`
	Vout          uint32 `gorm:"index"`
	Satoshis      int64

	LockingScript      []byte
	CustomInstructions *string `gorm:"type:string"`

	DerivationPrefix *string
	DerivationSuffix *string

	BasketName *string
	Basket     *OutputBasket `gorm:"foreignKey:UserID,BasketName;references:UserID,Name"`

	Spendable bool `gorm:"index"`
	Change    bool

	Description string `gorm:"type:string"`
	ProvidedBy  string
	Purpose     string
	Type        string

	SenderIdentityKey *string

	Transaction        *Transaction `gorm:"foreignKey:TransactionID;references:ID"`
	SpentByTransaction *Transaction `gorm:"foreignKey:SpentBy;references:ID"`

	UserUTXO *UserUTXO `gorm:"foreignKey:OutputID"`

	Tags []*Tag `gorm:"many2many:output_tags;"`
}
