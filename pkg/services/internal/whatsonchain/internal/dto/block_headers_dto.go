package dto

import (
	"fmt"
	"strconv"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// BlockHeader represents a Bitcoin block as returned by a block explorer or full node API.
//
// It includes core header fields, metadata about the block’s position and content,
// and references to adjacent blocks.
type BlockHeader struct {
	// Hash is the block's unique identifier, computed as the double SHA-256 hash of the serialized block header.
	Hash string `json:"hash"`

	// Height is the block’s position in the chain, starting from 0 (the genesis block).
	Height uint `json:"height"`

	// Version is is a 32-bit version number of the block.
	// It indicates which consensus rules or features are in effect.
	Version uint32 `json:"version"`

	// MerkleRoot is the root hash of the Merkle tree formed by all transactions in the block.
	// This value appears in the block header and proves transaction inclusion.
	MerkleRoot string `json:"merkleroot"`

	// Time is the block’s creation timestamp in Unix epoch seconds, set by the miner.
	Time uint32 `json:"time"`

	// Nonce is a 32-bit number that miners iterate to find a hash meeting the difficulty target.
	Nonce uint32 `json:"nonce"`

	// Bits is the compact, encoded representation of the difficulty target in hexadecimal.
	Bits string `json:"bits"`

	// PreviousBlockHash is the hash of the preceding block in the chain.
	PreviousBlockHash string `json:"previousblockhash"`
}

func (b *BlockHeader) IsZero() bool { return *b == BlockHeader{} }

func (b *BlockHeader) ConvertToChainBaseBlockHeader() (*wdk.ChainBaseBlockHeader, error) {
	bits, err := strconv.ParseUint(b.Bits, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid bits value %q: expected hex string convertible to uint32: %w", b.Bits, err)
	}

	return &wdk.ChainBaseBlockHeader{
		Version:      b.Version,
		PreviousHash: b.PreviousBlockHash,
		MerkleRoot:   b.MerkleRoot,
		Time:         b.Time,
		Bits:         uint32(bits),
		Nonce:        b.Nonce,
	}, nil
}

// ConvertToChainBlockHeader converts a BlockHeader into a *wdk.ChainBlockHeader used in chain processing.
func (b *BlockHeader) ConvertToChainBlockHeader() (*wdk.ChainBlockHeader, error) {
	base, err := b.ConvertToChainBaseBlockHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to convert BlockHeader to ChainBaseBlockHeader: %w", err)
	}

	return &wdk.ChainBlockHeader{
		ChainBaseBlockHeader: *base,
		Height:               b.Height,
		Hash:                 b.Hash,
	}, nil
}

// MerkleRootOnly represents the result of a Merkle root computation,
type MerkleRootOnly struct {
	MerkleRoot string `json:"merkleroot"`
}
