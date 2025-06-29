package testabilities

import (
	"encoding/hex"
	"log"

	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// MustHashFromHex creates a Hash from hex or panics on failure.
func MustHashFromHex(hex string) *chainhash.Hash {
	h, err := chainhash.NewHashFromHex(hex)
	if err != nil {
		log.Panicf("invalid hex for hash: %v", err)
	}
	return h
}

// FakeHeaderHexWithMerkleRoot returns a fake block header hex string with the given Merkle root in little-endian.
func FakeHeaderHexWithMerkleRoot(merkleRootHex string) string {
	header := make([]byte, TestBlockHeaderLength)

	merkleRootBytes, err := hex.DecodeString(merkleRootHex)
	if err != nil {
		log.Fatalf("failed to decode merkle root hex: %v", err)
	}

	if len(merkleRootBytes) != TestMerkleRootLength {
		log.Fatalf("invalid merkle root length: %d", len(merkleRootBytes))
	}

	for i := 0; i < TestMerkleRootLength; i++ {
		header[TestMerkleRootOffset+i] = merkleRootBytes[TestMerkleRootLength-1-i]
	}

	return hex.EncodeToString(header)
}
