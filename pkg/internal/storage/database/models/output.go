package models

import (
	"gorm.io/gorm"
)

type Output struct {
	gorm.Model

	UserID        int    `gorm:"index"`
	TransactionID uint   `gorm:"index"`
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
