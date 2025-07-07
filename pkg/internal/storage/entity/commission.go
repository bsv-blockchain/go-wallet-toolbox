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

var CommissionFieldNames = struct {
	ID            string
	CreatedAt     string
	UpdatedAt     string
	UserID        string
	TransactionID string
	Satoshis      string
	KeyOffset     string
	IsRedeemed    string
	LockingScript string
}{
	ID:            "id",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
	UserID:        "user_id",
	TransactionID: "transaction_id",
	Satoshis:      "satoshis",
	KeyOffset:     "key_offset",
	IsRedeemed:    "is_redeemed",
	LockingScript: "locking_script",
}
