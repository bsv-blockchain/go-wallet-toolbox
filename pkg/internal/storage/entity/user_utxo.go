package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type UserUTXO struct {
	UserID             int
	OutputID           uint
	BasketName         string
	Satoshis           uint64
	EstimatedInputSize uint64
	CreatedAt          time.Time
	ReservedByID       *uint
	Status             wdk.UTXOStatus
}
