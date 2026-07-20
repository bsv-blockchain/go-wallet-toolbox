package entity

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type ListActionsFilter struct {
	UserID         int
	Labels         []string
	LabelQueryMode defs.QueryMode
	Status         []wdk.TxStatus
	Limit          int
	Offset         int

	// BRC-114 action time filters (parsed from control labels).
	// CreatedAtFrom is inclusive; CreatedAtTo is exclusive.
	// Only consulted when TimeFilterRequested is true.
	TimeFilterRequested bool
	CreatedAtFrom       *time.Time
	CreatedAtTo         *time.Time
}
