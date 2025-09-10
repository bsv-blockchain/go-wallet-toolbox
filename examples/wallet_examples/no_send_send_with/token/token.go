package token

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/go-softwarelab/common/pkg/slices"
)

// Token represents a digital asset including transaction details, keys, and the amount in Satoshis.
type Token struct {
	TxID            chainhash.Hash
	Beef            []byte
	KeyID           string
	FromIdentityKey *ec.PublicKey
	Satoshis        uint64
}

// DataOutpoint returns a transaction.Outpoint with the TxID of the Token and an Index value of 0.
func (t Token) DataOutpoint() transaction.Outpoint {
	return transaction.Outpoint{
		Txid:  t.TxID,
		Index: 0,
	}
}

// Tokens represents a collection of Token objects, each encapsulating details about digital assets and their transactions.
type Tokens []Token

// TxIDs returns a slice of transaction IDs for each Token in the Tokens collection.
func (t Tokens) TxIDs() []chainhash.Hash {
	return slices.Map(t, func(token Token) chainhash.Hash {
		return token.TxID
	})
}
