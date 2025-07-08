package wdk

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TableCommission represents a commission entry with audit fields, user and transaction links, and payout details.
type TableCommission struct {
	CreatedAt     time.Time                    `json:"created_at"`
	UpdatedAt     time.Time                    `json:"updated_at"`
	CommissionID  uint                         `json:"commissionId"`
	UserID        int                          `json:"userId"`
	TransactionID uint                         `json:"transactionId"`
	Satoshis      int64                        `json:"satoshis"`
	KeyOffset     string                       `json:"keyOffset"`
	IsRedeemed    bool                         `json:"isRedeemed"`
	LockingScript primitives.ExplicitByteArray `json:"lockingScript"`
}
