package entity

import "github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"

type ListActionsFilter struct {
	UserID           int
	Labels           []string
	IncludeAllLabels bool // "any=false" or "all=true"
	Status           []wdk.TxStatus
	Limit            int
	Offset           int
}
