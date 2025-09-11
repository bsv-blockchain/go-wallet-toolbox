package services

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Named provides a structure to associate a name with any item of generic type T.
type Named[T any] struct {
	Name string
	Item T
}

// Possible service functions signatures.
type (
	RawTxFunc                        = func(ctx context.Context, txID string) (*wdk.RawTxResult, error)
	PostBEEFFunc                     = func(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error)
	MerklePathFunc                   = func(ctx context.Context, txID string) (*wdk.MerklePathResult, error)
	FindChainTipHeaderFunc           = func(ctx context.Context) (*wdk.ChainBlockHeader, error)
	IsValidRootForHeightServicesFunc = func(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error)
	CurrentHeightFunc                = func(ctx context.Context) (uint32, error)
	GetScriptHashHistoryFunc         = func(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error)
	HashToHeaderFunc                 = func(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error)
	ChainHeaderByHeightFunc          = func(ctx context.Context, height uint32) (*wdk.ChainBaseBlockHeader, error)
	GetStatusForTxIDsFunc            = func(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error)
	GetUtxoStatusFunc                = func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (*wdk.UtxoStatusResult, error)
	IsUtxo                           = func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (bool, error)
	BsvExchangeRateFunc              = func(ctx context.Context) (float64, error)
)

// Implementation defines all the methods that the services component supports.
// Each field corresponds to a specific service function.
// When a field is nil, it indicates that the particular service function doesn't have an implementation available - other services will be tried in that case.
type Implementation struct {
	RawTx                RawTxFunc
	PostBEEF             PostBEEFFunc
	MerklePath           MerklePathFunc
	FindChainTipHeader   FindChainTipHeaderFunc
	IsValidRootForHeight IsValidRootForHeightServicesFunc
	CurrentHeight        CurrentHeightFunc
	GetScriptHashHistory GetScriptHashHistoryFunc
	HashToHeader         HashToHeaderFunc
	ChainHeaderByHeight  ChainHeaderByHeightFunc
	GetStatusForTxIDs    GetStatusForTxIDsFunc
	GetUtxoStatus        GetUtxoStatusFunc
	IsUtxo               IsUtxo
	BsvExchangeRate      BsvExchangeRateFunc
}
