package models

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"

type LiveBlockHeader struct {
	wdk.ChainBlockHeader

	ChainWork        string
	IsChainTip       bool
	IsActive         bool
	HeaderID         uint
	PreviousHeaderID *uint
}
