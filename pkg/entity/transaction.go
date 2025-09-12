package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Transaction represents a blockchain transaction recorded in the system.
type Transaction struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time

	UserID      int
	Status      wdk.TxStatus
	Reference   string
	IsOutgoing  bool
	Satoshis    int64
	Description string
	Version     uint32
	LockTime    uint32
	TxID        *string
	InputBEEF   []byte
	Labels      []string
}

// TransactionReadSpecification defines filter criteria for querying transactions.
type TransactionReadSpecification struct {
	ID                  *uint
	UserID              *Comparable[int]
	Status              *Comparable[wdk.TxStatus]
	Reference           *Comparable[string]
	IsOutgoing          *Comparable[bool]
	Satoshis            *Comparable[int64]
	TxID                *Comparable[string]
	DescriptionContains *Comparable[string]
	Labels              *ComparableSet[string]
}

// TransactionUpdateSpecification defines fields that can be updated.
type TransactionUpdateSpecification struct {
	ID          uint
	Status      *wdk.TxStatus
	Description *string
}
