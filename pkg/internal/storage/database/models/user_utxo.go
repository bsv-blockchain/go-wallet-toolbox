package models

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
)

// UserUTXO is a table holding user's Unspent Transaction Outputs (UTXOs).
type UserUTXO struct {
	UserID   int     `gorm:"primaryKey"`
	OutputID uint    `gorm:"primaryKey"`
	Output   *Output `gorm:"foreignKey:OutputID"`

	BasketName string        `gorm:"not null,index"`
	Basket     *OutputBasket `gorm:"foreignKey:UserID,BasketName;references:UserID,Name"`

	Satoshis uint64
	// EstimatedInputSize is the estimated size increase when adding and unlocking this UTXO to a transaction.
	EstimatedInputSize uint64
	CreatedAt          time.Time

	ReservedByID *uint
	ReservedBy   *Transaction
}

func ToUserUTXOFromOutputEntity(output *entity.Output) *UserUTXO {
	userID := output.UserID

	var basket string
	if output.BasketName != nil {
		basket = *output.BasketName
	}

	return &UserUTXO{
		UserID:   userID,
		OutputID: output.ID,
		Output: &Output{
			UserID:             userID,
			TransactionID:      output.TransactionID,
			SpentBy:            output.SpentBy,
			Vout:               output.Vout,
			Satoshis:           output.Satoshis,
			LockingScript:      output.LockingScript,
			CustomInstructions: output.CustomInstructions,
			DerivationPrefix:   output.DerivationPrefix,
			DerivationSuffix:   output.DerivationSuffix,
			BasketName:         &basket,
			Basket: &OutputBasket{
				Name:   basket,
				UserID: userID,
			},
			Spendable:         output.Spendable,
			Change:            output.Change,
			Description:       output.Description,
			ProvidedBy:        output.ProvidedBy,
			Purpose:           output.Purpose,
			Type:              output.Type,
			SenderIdentityKey: output.SenderIdentityKey,
			Transaction: &Transaction{
				UserID: userID,
				TxID:   output.TxID,
			},
		},
		BasketName:         basket,
		Satoshis:           output.UserUTXO.Satoshis,
		EstimatedInputSize: output.UserUTXO.EstimatedInputSize,
		CreatedAt:          output.CreatedAt,
		ReservedByID:       output.UserUTXO.ReservedByID,
	}
}
