package wdk

import (
	"context"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// Services defines an interface for handling 3rd party services
type Services interface {
	chaintracker.ChainTracker

	PostBEEF(ctx context.Context, beef *transaction.Beef, txids []string) (PostBeefResult, error)
	MerklePath(ctx context.Context, txid string) (*MerklePathResult, error)
	FindChainTipHeader(ctx context.Context) (*ChainBlockHeader, error)
}
