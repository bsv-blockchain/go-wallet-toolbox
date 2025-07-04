package entity

import "time"

type Commission struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time

	UserID        int
	TransactionID uint
	Satoshis      uint64
	KeyOffset     string
	IsRedeemed    bool
	LockingScript []byte
}
