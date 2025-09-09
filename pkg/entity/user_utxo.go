package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// UserUTXO represents a UTXO owned by a user in the wallet.
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
