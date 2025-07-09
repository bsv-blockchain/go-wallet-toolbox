package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

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
