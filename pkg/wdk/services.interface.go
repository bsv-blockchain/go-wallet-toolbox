package wdk

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
)

// Services defines an interface for handling 3rd party services
type Services interface {
	BlockHeaderLoader
	chaintracker.ChainTracker
	PostBEEF(ctx context.Context, beef *transaction.Beef, txids []string) (PostBeefResult, error)
	MerklePath(ctx context.Context, txid string) (*MerklePathResult, error)
	FindChainTipHeader(ctx context.Context) (*ChainBlockHeader, error)
	RawTx(ctx context.Context, txID string) (RawTxResult, error)
	GetBEEF(ctx context.Context, txID string, knownTxIDs []string) (*transaction.Beef, error)
	NLockTimeIsFinal(ctx context.Context, txOrLockTime any) (bool, error)
	GetStatusForTxIDs(ctx context.Context, txIDs []string) (*GetStatusForTxIDsResult, error)
}

// HeightProvider is an interface that provides the current blockchain height.
type HeightProvider interface {
	CurrentHeight(ctx context.Context) (uint32, error)
}

// BlockHeaderLoader is an interface that provides the block chain block header with given height.
type BlockHeaderLoader interface {
	ChainHeaderByHeight(ctx context.Context, height uint32) (*ChainBlockHeader, error)
}
