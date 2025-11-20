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

// LiveOrBulkBlockHeader is an alias for LiveBlockHeader to be used in contexts where either live or bulk headers are applicable.
// In case of bulk header all the fields specific to the live header will be ignored. (all besides wdk.ChainBlockHeader)
type LiveOrBulkBlockHeader = LiveBlockHeader
