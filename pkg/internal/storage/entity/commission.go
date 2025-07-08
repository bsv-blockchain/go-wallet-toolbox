package entity

import (
	"time"
)

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

type CommissionReadSpecification struct {
	ID         *uint
	IsRedeemed *bool
	UserID     *int
	Satoshis   *ComparableNumber[uint64]
}

type CommissionUpdateSpecification struct {
	ID         uint
	IsRedeemed *bool
	Satoshis   *uint64
}
