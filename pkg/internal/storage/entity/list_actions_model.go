package entity

import (
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
	Reference      *string
}
