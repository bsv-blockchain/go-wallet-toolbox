package entity

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"time"
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
	InputBeef   []byte
}
