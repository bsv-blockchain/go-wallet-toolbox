package main

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slices"
)

type Token struct {
	wallet.CreateActionResult
	Beef            *transaction.Beef
	KeyID           string
	FromIdentityKey *ec.PublicKey
	Satoshis        uint64
}

func (t Token) DataOutpoint() transaction.Outpoint {
	return transaction.Outpoint{
		Txid: t.Txid,
		Index: 0,
	}
}

type Tokens []Token

func (t Tokens) TxIDs() []chainhash.Hash {
	return slices.Map(t, func(token Token) chainhash.Hash {
		return token.Txid
	})
}
