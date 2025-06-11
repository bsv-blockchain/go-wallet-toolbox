package whatsonchain

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// UtxoStatusOutputFormat represents supported utxo status output formats
type UtxoStatusOutputFormat string

// Supported utxo status output formats
const (
	HashLE UtxoStatusOutputFormat = "hashLE"
	HashBE UtxoStatusOutputFormat = "hashBE"
	Script UtxoStatusOutputFormat = "script"
)

// BaseBlockHeader are fields of 80 byte serialized header in order whose double sha256 hash is a block's hash value
// and the next block's previousHash value.
// All block hash values and merkleRoot values are 32 byte hex string values with the byte order reversed from the serialized byte order.
type BaseBlockHeader struct {
	// Block header version value. Serialized length is 4 bytes.
	Version int64
	// PreviousHash is a hash of previous block's block header. Serialized length is 32 bytes.
	PreviousHash string
	// MerkleRoot is root hash of the merkle tree of all transactions in this block. Serialized length is 32 bytes.
	MerkleRoot string
	// Time is block header time value. Serialized length is 4 bytes.
	Time int64
	// Bits are block header bits value. Serialized length is 4 bytes.
	Bits int64
	// Nonce is block header nonce value. Serialized length is 4 bytes.
	Nonce int64
}

// BlockHeader is a base block header with its computed height and hash in its chain
type BlockHeader struct {
	BaseBlockHeader
	// Height is the of the header, starting from zero
	Height uint
	// Hash is the double sha256 hash of the serialized `BaseBlockHeader` fields
	Hash string
}

// MerklePathResult is result from MerklePath method
type MerklePathResult struct {
	// Name is the name of the service returning the proof, or undefined if no proof
	Name *string
	// MerklePath are multiple proofs may be returned when a transaction also appears in
	// one or more orphaned blocks
	MerklePath *transaction.MerklePath
	Header     *BlockHeader
	Notes      []wdk.ReqHistoryNote
}

// UtxoStatusDetails represents details about occurrences of an output script as a UTXO
type UtxoStatusDetails struct {
	// Height is the block height containing the matching unspent transaction output
	// Typically there will be only one, but future orphans can result in multiple values
	Height *int64

	// Txid is the transaction hash (txid) of the transaction containing the matching unspent transaction output
	// Typically there will be only one, but future orphans can result in multiple values
	Txid *string

	// Index is the output index in the transaction containing of the matching unspent transaction output
	// Typically there will be only one, but future orphans can result in multiple values
	Index *int64

	// Satoshis is the amount of the matching unspent transaction output
	// Typically there will be only one, but future orphans can result in multiple values
	Satoshis *uint64
}

// UtxoStatusResult represents the result of a GetUtxoStatus operation
type UtxoStatusResult struct {
	// Name is the name of the service to which the transaction was submitted for processing
	Name string

	// IsUtxo is true if the output is associated with at least one unspent transaction output
	IsUtxo *bool

	// Details contains additional details about occurrences of this output script as a UTXO.
	// Normally there will be one item in the array but due to the possibility of orphan races
	// there could be more than one block in which it is a valid UTXO.
	Details []UtxoStatusDetails
}
