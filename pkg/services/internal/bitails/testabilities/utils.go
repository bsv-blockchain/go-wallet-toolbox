package testabilities

import (
	"encoding/hex"
	"testing"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/require"
)

// HashFromHex creates a *chainhash.Hash from a hex string.
// It marks the helper, asserts no error, and returns the hash.
func HashFromHex(t testing.TB, hexStr string) *chainhash.Hash {
	t.Helper()

	h, err := chainhash.NewHashFromHex(hexStr)
	require.NoError(t, err, "invalid hex for hash")
	require.NotNil(t, h, "hash must not be nil")

	return h
}

// FakeHeaderHexWithMerkleRoot builds a fake 80-byte block header hex string
// with the supplied merkle-root.  The headers version/time/nonce
// fields are all zero only the merkle-root matters for tests.
func FakeHeaderHexWithMerkleRoot(t testing.TB, merkleRootHex string) string {
	t.Helper()

	header := make([]byte, TestBlockHeaderLength) // 80 bytes
	merkleRootBytes, err := hex.DecodeString(merkleRootHex)
	require.NoError(t, err, "cannot decode merkle root hex")
	require.Equal(t, TestMerkleRootLength, len(merkleRootBytes), "merkle root must be 32 bytes")

	for i := 0; i < TestMerkleRootLength; i++ {
		header[TestMerkleRootOffset+i] = merkleRootBytes[TestMerkleRootLength-1-i]
	}
	return hex.EncodeToString(header)
}
