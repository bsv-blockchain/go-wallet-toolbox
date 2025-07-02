package fixtures

import (
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

const DefaultOriginator = "tests"

func MockTransactionOutpoint() transaction.Outpoint {
	txid, _ := chainhash.NewHashFromHex("0000000000000000000000000000000000000000000000000000000000000000")
	return transaction.Outpoint{
		Txid:  *txid,
		Index: 0,
	}
}
