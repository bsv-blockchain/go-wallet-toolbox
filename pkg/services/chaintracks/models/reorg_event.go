package models

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"

// ReorgEvent represents a chain reorganization event involving a change from one chain tip to another.
type ReorgEvent struct {
	OldTip wdk.ChainBlockHeader
	NewTip   wdk.ChainBlockHeader
}
