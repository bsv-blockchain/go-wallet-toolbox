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

type (
	// RawTxFunc is a function type for RawTx service method.
	RawTxFunc = func(ctx context.Context, txID string) (*wdk.RawTxResult, error)
	// PostBEEFFunc is a function type for PostBEEF service method.
	PostBEEFFunc = func(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error)
	// MerklePathFunc is a function type for MerklePath service method.
	MerklePathFunc = func(ctx context.Context, txID string) (*wdk.MerklePathResult, error)
	// FindChainTipHeaderFunc is a function type for FindChainTipHeader service method.
	FindChainTipHeaderFunc = func(ctx context.Context) (*wdk.ChainBlockHeader, error)
	// IsValidRootForHeightFunc is a function type for IsValidRootForHeight service method.
	IsValidRootForHeightFunc = func(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error)
	// CurrentHeightFunc is a function type for CurrentHeight service method.
	CurrentHeightFunc = func(ctx context.Context) (uint32, error)
	// GetScriptHashHistoryFunc is a function type for GetScriptHashHistory service method.
	GetScriptHashHistoryFunc = func(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error)
	// HashToHeaderFunc is a function type for HashToHeader service method.
	HashToHeaderFunc = func(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error)
	// ChainHeaderByHeightFunc is a function type for ChainHeaderByHeight service method.
	ChainHeaderByHeightFunc = func(ctx context.Context, height uint32) (*wdk.ChainBaseBlockHeader, error)
	// GetStatusForTxIDsFunc is a function type for GetStatusForTxIDs service method.
	GetStatusForTxIDsFunc = func(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error)
	// GetUtxoStatusFunc is a function type for GetUtxoStatus service method.
	GetUtxoStatusFunc = func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (*wdk.UtxoStatusResult, error)
	// IsUtxo is a function type for IsUtxo service method.
	IsUtxo = func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (bool, error)
	// BsvExchangeRateFunc is a function type for BsvExchangeRate service method.
	BsvExchangeRateFunc = func(ctx context.Context) (float64, error)
)

// Implementation defines all the methods that the services component supports.
// Each field corresponds to a specific service function.
// When a field is nil, it indicates that the particular service function doesn't have an implementation available - other services will be tried in that case.
type Implementation struct {
	RawTx                RawTxFunc
	PostBEEF             PostBEEFFunc
	MerklePath           MerklePathFunc
	FindChainTipHeader   FindChainTipHeaderFunc
	IsValidRootForHeight IsValidRootForHeightFunc
	CurrentHeight        CurrentHeightFunc
	GetScriptHashHistory GetScriptHashHistoryFunc
	HashToHeader         HashToHeaderFunc
	ChainHeaderByHeight  ChainHeaderByHeightFunc
	GetStatusForTxIDs    GetStatusForTxIDsFunc
	GetUtxoStatus        GetUtxoStatusFunc
	IsUtxo               IsUtxo
	BsvExchangeRate      BsvExchangeRateFunc
}
