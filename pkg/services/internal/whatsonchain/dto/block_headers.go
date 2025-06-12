package dto

import (
	"fmt"
	"strconv"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

// BlockHeaders represents a list of BlockHeader objects that represent the structure of a blockchain block as returned by a block explorer or node API.
type BlockHeaders []BlockHeader

// First returns the first BlockHeader in the slice.
//
// This method is useful when only the earliest (or latest, depending on sort order) block
// is of interest, such as when querying for the most recent block.
//
// It panics if the slice is nil or has zero elements. Use IsEmpty to check before calling
// if the slice may be empty.
func (bb BlockHeaders) First() BlockHeader {
	if bb.IsEmpty() {
		panic("attempted to get a first element from nil or zero length slice")
	}
	return bb[0]
}

// IsEmpty returns true if the BlockHeaders slice has no elements or is nil.
//
// This is a convenience method to check for empty results from block queries,
// and can be used to avoid panics when accessing elements of the slice.
func (bb BlockHeaders) IsEmpty() bool {
	return len(bb) == 0
}

// BlockHeader represents a Bitcoin block as returned by a block explorer or full node API.
//
// It includes core header fields, metadata about the block’s position and content,
// and references to adjacent blocks.
type BlockHeader struct {
	// Hash is the block's unique identifier, computed as the double SHA-256 hash of the serialized block header.
	Hash string `json:"hash"`

	// Confirmations is the number of blocks added on top of this block.
	// A higher number indicates deeper confirmation in the blockchain.
	Confirmations uint `json:"confirmations"`

	// Size is the total size of the block in bytes, including header and transactions.
	Size uint `json:"size"`

	// Height is the block’s position in the chain, starting from 0 (the genesis block).
	Height uint `json:"height"`

	// Version is the integer version number of the block.
	// It indicates which consensus rules or features are in effect.
	Version uint64 `json:"version"`

	// VersionHex is the hexadecimal-encoded version field, matching the raw serialized representation.
	VersionHex string `json:"versionHex"`

	// MerkleRoot is the root hash of the Merkle tree formed by all transactions in the block.
	// This value appears in the block header and proves transaction inclusion.
	MerkleRoot string `json:"merkleroot"`

	// Time is the block’s creation timestamp in Unix epoch seconds, set by the miner.
	Time uint64 `json:"time"`

	// Mediantime is the median timestamp of the previous 11 blocks.
	// It is used for enforcing time-based rules like locktime.
	Mediantime uint64 `json:"mediantime"`

	// Nonce is a 32-bit number that miners iterate to find a hash meeting the difficulty target.
	Nonce uint64 `json:"nonce"`

	// Bits is the compact, encoded representation of the difficulty target in hexadecimal.
	Bits string `json:"bits"`

	// Difficulty is the actual mining difficulty as a floating-point number.
	// It represents how hard it was to mine the block relative to the easiest possible difficulty.
	Difficulty float64 `json:"difficulty"`

	// Chainwork is a hex string representing the total cumulative proof-of-work in the chain up to and including this block.
	Chainwork string `json:"chainwork"`

	// PreviousBlockHash is the hash of the preceding block in the chain.
	PreviousBlockHash string `json:"previousblockhash"`

	// NextBlockHash is the hash of the succeeding block, if it exists.
	// It may be empty if this block is the current chain tip.
	NextBlockHash string `json:"nextblockhash"`

	// NTx is the reported number of transactions in the block (may be a legacy or alias field).
	NTx int `json:"nTx"`

	// NumTx is the number of transactions in the block.
	// It may be equal to or override NTx depending on the API source.
	NumTx int `json:"num_tx"`
}

// ConvertToChainBlockHeader converts a BlockHeader into a *wdk.ChainBlockHeader used in chain processing.
//
// It parses the `Bits` field from a hexadecimal string to a uint64 integer.
// Returns an error if the conversion fails.
func (b *BlockHeader) ConvertToChainBlockHeader() (*wdk.ChainBlockHeader, error) {
	bits, err := strconv.ParseUint(b.Bits, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid bits value %q: expected hex string convertible to uint64: %w", b.Bits, err)
	}

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: wdk.ChainBaseBlockHeader{
			Version:      b.Version,
			PreviousHash: b.PreviousBlockHash,
			MerkleRoot:   b.MerkleRoot,
			Time:         b.Time,
			Bits:         bits,
			Nonce:        b.Nonce,
		},
		Height: b.Height,
		Hash:   b.Hash,
	}, nil
}
