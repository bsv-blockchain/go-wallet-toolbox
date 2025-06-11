package dto

// BlockResponses represents a list of BlockResponse objects that represent the structure of a blockchain block as returned by a block explorer or node API.
type BlockResponses []BlockResponse

// First returns the first BlockResponse in the slice.
//
// This method is useful when only the earliest (or latest, depending on sort order) block
// is of interest, such as when querying for the most recent block.
//
// It panics if the slice is nil or has zero elements. Use IsEmpty to check before calling
// if the slice may be empty.
func (bb BlockResponses) First() BlockResponse {
	if bb.IsEmpty() {
		panic("attempted to get a first element from nil or zero length slice")
	}
	return bb[0]
}

// IsEmpty returns true if the BlockResponses slice has no elements or is nil.
//
// This is a convenience method to check for empty results from block queries,
// and can be used to avoid panics when accessing elements of the slice.
func (bb BlockResponses) IsEmpty() bool {
	return len(bb) == 0
}

// BlockResponse represents the structure of a blockchain block as returned by a block explorer or node API.
type BlockResponse struct {
	// Hash is the block's unique identifier (block hash).
	Hash string `json:"hash"`

	// Confirmations is the number of blocks added after this block, indicating how deep it is in the chain.
	Confirmations int `json:"confirmations"`

	// Size is the total size of the block in bytes.
	Size int `json:"size"`

	// Height is the block's position in the blockchain (starting from 0 for the genesis block).
	Height int `json:"height"`

	// Version is the block version as an integer.
	Version int `json:"version"`

	// VersionHex is the hexadecimal representation of the block version.
	VersionHex string `json:"versionHex"`

	// MerkleRoot is the root of the Merkle tree containing all transaction hashes in the block.
	MerkleRoot string `json:"merkleroot"`

	// Time is the block timestamp in UNIX epoch seconds.
	Time int64 `json:"time"`

	// Mediantime is the median of the timestamps of the previous 11 blocks.
	Mediantime int64 `json:"mediantime"`

	// Nonce is the nonce value that led to a successful proof of work for this block.
	Nonce uint32 `json:"nonce"`

	// Bits is the compact representation of the target difficulty in hexadecimal.
	Bits string `json:"bits"`

	// Difficulty is the actual calculated difficulty value for mining the block.
	Difficulty float64 `json:"difficulty"`

	// Chainwork is a hexadecimal representation of the total cumulative work in the chain up to this block.
	Chainwork string `json:"chainwork"`

	// PreviousBlockHash is the hash of the preceding block in the chain.
	PreviousBlockHash string `json:"previousblockhash"`

	// NextBlockHash is the hash of the next block (if it exists); may be empty if this is the chain tip.
	NextBlockHash string `json:"nextblockhash"`

	// NTx is an alternative field representing the number of transactions in the block.
	NTx int `json:"nTx"`

	// NumTx is the total number of transactions in the block; may duplicate or differ from NTx depending on the API.
	NumTx int `json:"num_tx"`
}
