package wdk

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// MerklePathResult is the result of a MerklePath query
type MerklePathResult struct {
	// Name is the name of the service returning the rawTx or nil if no rawTx
	Name string

	// MerklePath is the MerklePath of the transaction
	MerklePath *transaction.MerklePath

	// Header is the header of the block containing the transaction
	Header *BlockHeader

	// Notes are the service debug notes for processing the request
	Notes Notes
}

// BlockHeader is the header of a block
type BlockHeader struct {
	// Height is the of the header, starting from zero
	Height uint32
	// Hash is the double sha256 hash of the serialized `BaseBlockHeader` fields
	Hash string
}
