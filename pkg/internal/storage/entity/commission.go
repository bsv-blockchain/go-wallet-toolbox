package entity

import (
	"github.com/go-softwarelab/common/pkg/types"
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

type CommissionSpecification struct {
	ID         *uint
	IsRedeemed *bool
	Satoshis   *ComparableNumber[uint64]
}

type NumberCmpOperator string

const (
	GreaterThan        NumberCmpOperator = ">"
	LessThan           NumberCmpOperator = "<"
	Equal              NumberCmpOperator = "="
	NotEqual           NumberCmpOperator = "!="
	GreaterThanOrEqual NumberCmpOperator = ">="
	LessThanOrEqual    NumberCmpOperator = "<="
)

type ComparableNumber[T types.Number] struct {
	Value T
	Cmp   NumberCmpOperator
}
