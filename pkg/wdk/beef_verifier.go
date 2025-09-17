package wdk

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// BeefVerifier defines an interface for verifying beef transactions in a blockchain context.
// The VerifyBeef method checks the validity of a beef transaction using contextual and chain data.
// Implementations may use the allowTxidOnly flag to restrict verification based on txid presence only.
type BeefVerifier interface {
	VerifyBeef(ctx context.Context, beef *transaction.Beef, allowTxidOnly bool) (bool, error)
}
