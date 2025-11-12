package models

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"

// LiveBlockHeader represents a blockchain header with live chain metadata and relational connectivity between headers.
type LiveBlockHeader struct {
	wdk.ChainBlockHeader

	ChainWork        string
	IsChainTip       bool
	IsActive         bool
	HeaderID         uint
	PreviousHeaderID *uint
}
