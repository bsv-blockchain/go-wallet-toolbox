package wdk

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// ChainBaseBlockHeader represents the raw fields of a Bitcoin block header,
// corresponding to the 80-byte serialized format used in the Bitcoin protocol.
// The double SHA-256 hash of this serialized data is the block's identifier (hash)
// and is referenced by the `previousHash` field of the next block.
//
// Note: Both the `PreviousHash` and `MerkleRoot` fields are 32-byte hex strings
// with reversed byte order compared to their serialized binary format.
type ChainBaseBlockHeader struct {
	// Version is the 32-bit block header version. Serialized as 4 bytes (little-endian).
	Version uint32

	// PreviousHash is the hash of the previous block’s header.
	// Represented as a 32-byte hex string with reversed byte order.
	// Serialized length: 32 bytes.
	PreviousHash string

	// MerkleRoot is the root hash of the Merkle tree of all transactions in this block.
	// Represented as a 32-byte hex string with reversed byte order.
	// Serialized length: 32 bytes.
	MerkleRoot string

	// Time is the Unix timestamp indicating when the block was created.
	// Serialized as 4 bytes.
	Time uint32

	// Bits represents the compact encoding of the block's target difficulty.
	// Serialized as 4 bytes.
	Bits uint32

	// Nonce is the 32-bit nonce used in the mining process to vary the block hash.
	// Serialized as 4 bytes.
	Nonce uint32
}

// ChainBlockHeader extends ChainBaseBlockHeader with metadata about the block's
// position in the chain and its computed hash.
type ChainBlockHeader struct {
	ChainBaseBlockHeader

	// Height is the block's position in the blockchain, starting from 0 (the genesis block).
	Height uint

	// Hash is the double SHA-256 hash of the serialized block header.
	// Represented as a 32-byte hex string with reversed byte order.
	Hash string
}

// Hex returns the hexadecimal string representation of the block header.
// It marshals the block header fields into a byte slice and encodes it as a hex string.
// Returns an error if marshaling fails.
func (c *ChainBaseBlockHeader) Hex() (string, error) {
	bb, err := c.Bytes()
	if err != nil {
		return "", fmt.Errorf("failed to marshal chain block header: %w", err)
	}
	return hex.EncodeToString(bb), nil
}

// Bytes returns the serialized byte representation of the block header.
// It includes the reversed previous block hash and Merkle root,
// followed by time, bits, and nonce fields written in little-endian order.
// Returns an error if any of the fields cannot be parsed or written.
func (c *ChainBaseBlockHeader) Bytes() ([]byte, error) {
	hash, err := hex.DecodeString(c.PreviousHash)
	if err != nil {
		return nil, fmt.Errorf("failed to convert 'previous hash' field into bytes slice: %w", err)
	}

	if len(hash) != 32 {
		return nil, fmt.Errorf("'previous hash' field should be a 32 byte-hex length")
	}

	root, err := hex.DecodeString(c.MerkleRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to convert 'merkle root' field into bytes slice: %w", err)
	}

	if len(root) != 32 {
		return nil, fmt.Errorf("'merkle root' field should be a 32 byte-hex length")
	}

	buff := bytes.NewBuffer(make([]byte, 0, 80))
	if err := writeLittleEndianOrder(buff, c.Version); err != nil {
		return nil, fmt.Errorf("failed to write the 'version' field bytes in little-endian order: %w", err)
	}
	if err := writeReversedBytes(buff, hash); err != nil {
		return nil, fmt.Errorf("failed to write the 'previous hash' field bytes in little-endian order: %w", err)
	}
	if err := writeReversedBytes(buff, root); err != nil {
		return nil, fmt.Errorf("failed to write the 'merkle root' field bytes in little-endian order: %w", err)
	}
	if err := writeLittleEndianOrder(buff, c.Time); err != nil {
		return nil, fmt.Errorf("failed to write the 'time' field bytes in little-endian order: %w", err)
	}
	if err := writeLittleEndianOrder(buff, c.Bits); err != nil {
		return nil, fmt.Errorf("failed to write the 'bits' field bytes in little-endian order: %w", err)
	}
	if err := writeLittleEndianOrder(buff, c.Nonce); err != nil {
		return nil, fmt.Errorf("failed to write the 'nonce' field bytes in little-endian order: %w", err)
	}

	return buff.Bytes(), nil
}

func writeReversedBytes(buff *bytes.Buffer, data []byte) error {
	for i := len(data) - 1; i >= 0; i-- {
		if err := buff.WriteByte(data[i]); err != nil {
			return fmt.Errorf("failed to write byte %d of data '%x' : %w", i, data, err)
		}
	}
	return nil
}

func writeLittleEndianOrder(buff *bytes.Buffer, v any) error {
	if err := binary.Write(buff, binary.LittleEndian, v); err != nil {
		return fmt.Errorf("failed to write the binary representation of data '%v' to buffer: %w", v, err)
	}
	return nil
}
