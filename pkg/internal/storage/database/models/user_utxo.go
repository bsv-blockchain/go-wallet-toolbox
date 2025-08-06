package models

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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
		BasketName:         basket,
		Satoshis:           uint64(output.Satoshis),
		EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
		CreatedAt:          output.CreatedAt,
	}
}
