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
	require.Len(t, merkleRootBytes, TestMerkleRootLength, "merkle root must be 32 bytes")

	for i := 0; i < TestMerkleRootLength; i++ {
		header[TestMerkleRootOffset+i] = merkleRootBytes[TestMerkleRootLength-1-i]
	}
	return hex.EncodeToString(header)
}

// ValidBlockHeaderRaw returns a valid 80-byte block header in hex format.
func ValidBlockHeaderRaw() string {
	return "010000000c59cf62add14129195d91b7e55dad81b539002d7366acfc01902c0000000000ec5abb8c8b90e2e04c14648853ba9d262e4e8677b374a7da52650f2ea5ea1a9275f2794c9820691b0689738c"
}

// BlockHeaderRawWithInvalidBits returns a valid 80-byte block header in hex format
func BlockHeaderRawWithInvalidBits() string {
	return "00000020aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaINVALIDBITS"
}

// IncompleteBlockHeaderRaw returns a shorter than expected block header hex string.
func IncompleteBlockHeaderRaw() string {
	// Shorter than expected header hex (just 40 chars instead of 160)
	return "00000020aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}

// MustDecodeHex decodes a hex string or panics if invalid.
func MustDecodeHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err, "failed to decode hex string")
	return b
}
