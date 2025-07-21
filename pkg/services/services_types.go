package services

import (
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// UtxoStatusOutputFormat represents supported utxo status output formats
type UtxoStatusOutputFormat string

// Supported utxo status output formats
const (
	HashLE UtxoStatusOutputFormat = "hashLE"
	HashBE UtxoStatusOutputFormat = "hashBE"
	Script UtxoStatusOutputFormat = "script"
)

// MerklePathResult is result from MerklePath method
type MerklePathResult struct {
	// Name is the name of the service returning the proof, or undefined if no proof
	Name *string
	// MerklePath are multiple proofs may be returned when a transaction also appears in
	// one or more orphaned blocks
	MerklePath *transaction.MerklePath
	Header     *wdk.ChainBlockHeader
	Notes      wdk.HistoryNotes
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
