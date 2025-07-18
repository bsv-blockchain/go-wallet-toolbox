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

	// BlockHeader is the header of the block containing the transaction
	BlockHeader *MerklePathBlockHeader

	// Notes are the service debug notes for processing the request
	Notes HistoryNotes
}

// MerklePathBlockHeader is the header of a block
type MerklePathBlockHeader struct {
	// Height is the of the header, starting from zero
	Height uint32

	// MerkleRoot is the hexadecimal string representation of the Merkle tree root for all transactions in the block.
	MerkleRoot string

	// Hash is the hexadecimal string representation of the block hash
	Hash string
}
