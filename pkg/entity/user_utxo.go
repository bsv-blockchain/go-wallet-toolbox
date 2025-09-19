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

// UserUTXOReadSpecification defines the read specification for UserUTXO.
type UserUTXOReadSpecification struct {
	UserID             *int
	OutputID           *Comparable[uint]
	BasketName         *Comparable[string]
	Status             *Comparable[wdk.UTXOStatus]
	Satoshis           *Comparable[uint64]
	EstimatedInputSize *Comparable[uint64]
	ReservedByID       *Comparable[uint]
}

// UserUTXOUpdateSpecification defines the update specification for UserUTXO.
type UserUTXOUpdateSpecification struct {
	UserID             *int
	EstimatedInputSize *uint64
	Satoshis           *uint64
	OutputID           uint
	ReservedByID       *uint
	Status             *wdk.UTXOStatus
	BasketName         *string
}
