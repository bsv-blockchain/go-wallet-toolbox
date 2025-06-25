package entity

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type ListActionsFilter struct {
	UserID         int
	Labels         []string
	LabelQueryMode defs.QueryMode
	Status         []wdk.TxStatus
	Limit          int
	Offset         int
}
