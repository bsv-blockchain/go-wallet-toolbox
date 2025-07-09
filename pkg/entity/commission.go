package entity

import (
	"time"
)

// Commission represents a record of commission allocated on a transaction for a specific user.
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

// CommissionReadSpecification defines filter criteria for querying commissions based on ID, redemption, user, or satoshi amount.
type CommissionReadSpecification struct {
	ID            *uint
	IsRedeemed    *bool
	UserID        *int
	Satoshis      *Comparable[uint64]
	TransactionID *Comparable[uint]
	KeyOffset     *Comparable[string]
}

// CommissionUpdateSpecification defines the fields for updating a commission record in persistent storage.
// Only non-nil fields are updated, allowing for partial updates of commission records.
type CommissionUpdateSpecification struct {
	ID         uint
	IsRedeemed *bool
}
