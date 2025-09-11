package services

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Named[T any] struct {
	Name string
	Item T
}

type RawTxFunc = func(ctx context.Context, txID string) (*wdk.RawTxResult, error)
type PostBEEFFunc = func(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error)
type MerklePathFunc = func(ctx context.Context, txID string) (*wdk.MerklePathResult, error)
type FindChainTipHeaderFunc = func(ctx context.Context) (*wdk.ChainBlockHeader, error)
type IsValidRootForHeightServicesFunc = func(ctx context.Context, root *chainhash.Hash, height uint32) (bool, error)
type CurrentHeightFunc = func(ctx context.Context) (uint32, error)
type GetScriptHashHistoryFunc = func(ctx context.Context, scriptHash string) (*wdk.ScriptHistoryResult, error)
type HashToHeaderFunc = func(ctx context.Context, hash string) (*wdk.ChainBlockHeader, error)
type ChainHeaderByHeightFunc = func(ctx context.Context, height uint32) (*wdk.ChainBaseBlockHeader, error)
type GetStatusForTxIDsFunc = func(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error)
type GetUtxoStatusFunc = func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (*wdk.UtxoStatusResult, error)
type IsUtxo = func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (bool, error)
type BsvExchangeRateFunc = func(ctx context.Context) (float64, error)

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
