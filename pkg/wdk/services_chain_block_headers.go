package wdk

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
)

// BlockHeaderLength defines the fixed byte size of a block header, commonly used in blockchain data structures.
const BlockHeaderLength = 80

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

// Validate checks if PreviousHash and MerkleRoot are valid 32-byte hex strings (64 characters) and returns an error if not.
func (c *ChainBaseBlockHeader) Validate() error {
	if len(c.PreviousHash) != 64 {
		return fmt.Errorf("previous hash must be a 32-byte hex string (64 characters), got %d characters", len(c.PreviousHash))
	}

	if len(c.MerkleRoot) != 64 {
		return fmt.Errorf("merkle root must be a 32-byte hex string (64 characters), got %d characters", len(c.MerkleRoot))
	}

	return nil
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

// Validate checks the integrity of the ChainBlockHeader fields and returns an error if any are invalid.
// It validates the embedded ChainBaseBlockHeader and ensures the Hash field is a 32-byte hex string (64 characters).
func (c *ChainBlockHeader) Validate() error {
	if err := c.ChainBaseBlockHeader.Validate(); err != nil {
		return err
	}

	if len(c.Hash) != 64 {
		return fmt.Errorf("hash must be a 32-byte hex string (64 characters), got %d characters", len(c.Hash))
	}

	return nil
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

// CalculateHash computes and returns the double SHA-256 hash of the block header.
// Returns an error if serialization or hashing fails.
func (c *ChainBaseBlockHeader) CalculateHash() (chainhash.Hash, error) {
	bb, err := c.Bytes()
	if err != nil {
		return chainhash.Hash{}, fmt.Errorf("failed to marshal chain block header: %w", err)
	}

	return chainhash.DoubleHashH(bb), nil
}

// Bytes returns the serialized byte representation of the block header.
// It includes the reversed previous block hash and Merkle root,
// followed by time, bits, and nonce fields written in little-endian order.
// Returns an error if any of the fields cannot be parsed or written.
func (c *ChainBaseBlockHeader) Bytes() ([]byte, error) {
	var prevHash []byte
	var err error

	genesis := c.PreviousHash == ""

	if genesis {
		prevHash = make([]byte, 32)
	} else {
		prevHash, err = hex.DecodeString(c.PreviousHash)
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'previous hash' field into bytes slice: %w", err)
		}

		if len(prevHash) != 32 {
			return nil, fmt.Errorf("'previous hash' field should be a 32 byte-hex length")
		}
	}

	root, err := hex.DecodeString(c.MerkleRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to convert 'merkle root' field into bytes slice: %w", err)
	}

	if len(root) != 32 {
		return nil, fmt.Errorf("'merkle root' field should be a 32 byte-hex length")
	}

	buff := bytes.NewBuffer(make([]byte, 0, BlockHeaderLength))
	if err := writeLittleEndianOrder(buff, c.Version); err != nil {
		return nil, fmt.Errorf("failed to write the 'version' field bytes in little-endian order: %w", err)
	}
	if err := writeReversedBytes(buff, prevHash); err != nil {
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

func readReversedBytes(buff *bytes.Buffer, length int) ([]byte, error) {
	data := make([]byte, length)
	for i := length - 1; i >= 0; i-- {
		b, err := buff.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read byte %d of data: %w", i, err)
		}
		data[i] = b
	}
	return data, nil
}

func readLittleEndianOrder[T any](buff *bytes.Buffer) (T, error) {
	var v T
	if err := binary.Read(buff, binary.LittleEndian, &v); err != nil {
		return v, fmt.Errorf("failed to read the binary representation of data into type '%T': %w", v, err)
	}
	return v, nil
}

// ChainBaseBlockHeaderFromBytes parses an 80-byte slice as a Bitcoin block header and returns a ChainBaseBlockHeader.
// Returns an error if the byte slice is not exactly 80 bytes or if decoding individual fields fails.
func ChainBaseBlockHeaderFromBytes(data []byte) (*ChainBaseBlockHeader, error) {
	if len(data) != BlockHeaderLength {
		return nil, fmt.Errorf("data length %d is not equal to block header length %d", len(data), BlockHeaderLength)
	}

	buff := bytes.NewBuffer(data)

	version, err := readLittleEndianOrder[uint32](buff)
	if err != nil {
		return nil, fmt.Errorf("failed to read 'version' field from data: %w", err)
	}

	prevHashBytes, err := readReversedBytes(buff, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to read 'previous hash' field from data: %w", err)
	}
	prevHash := hex.EncodeToString(prevHashBytes)

	merkleRootBytes, err := readReversedBytes(buff, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to read 'merkle root' field from data: %w", err)
	}
	merkleRoot := hex.EncodeToString(merkleRootBytes)

	time, err := readLittleEndianOrder[uint32](buff)
	if err != nil {
		return nil, fmt.Errorf("failed to read 'time' field from data: %w", err)
	}

	bits, err := readLittleEndianOrder[uint32](buff)
	if err != nil {
		return nil, fmt.Errorf("failed to read 'bits' field from data: %w", err)
	}

	nonce, err := readLittleEndianOrder[uint32](buff)
	if err != nil {
		return nil, fmt.Errorf("failed to read 'nonce' field from data: %w", err)
	}

	return &ChainBaseBlockHeader{
		Version:      version,
		PreviousHash: prevHash,
		MerkleRoot:   merkleRoot,
		Time:         time,
		Bits:         bits,
		Nonce:        nonce,
	}, nil
}
