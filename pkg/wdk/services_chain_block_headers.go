package wdk

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
	Time uint64

	// Bits represents the compact encoding of the block's target difficulty.
	// Serialized as 4 bytes.
	Bits uint64

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
