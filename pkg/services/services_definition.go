package services

import (
	"context"
	"reflect"

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
	// PostTXFunc is a function type for PostTX service method (broadcasts raw transaction).
	PostTXFunc = func(ctx context.Context, rawTx []byte) (*wdk.PostedTxID, error)
	// PostEFFunc is a function type for PostEF service method (broadcasts EF format transaction).
	PostEFFunc = func(ctx context.Context, efHex, txID string) (*wdk.PostedTxID, error)
	// PostEFBatchFunc is a function type for the PostEFBatch service method: it broadcasts
	// transactions in extended format, as bytes, all in one call. It returns one result per
	// transaction, in the order given, or an error when the call as a whole failed.
	//
	// An implementation's PostEFBatch takes the place of its PostEF on the broadcast-to-all
	// path (Arcade disabled): all transactions of a PostFromBEEF call go to it in one call,
	// parents first, and an implementation that sets both is called only through PostEFBatch.
	// When a PostEFMethodsModifier is set, it is listed among the PostEF methods instead, as a
	// PostEF of one transaction, so the modifier still sees and controls every broadcaster.
	PostEFBatchFunc = func(ctx context.Context, txs []EFTransaction) ([]*wdk.PostedTxID, error)
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
	ChainHeaderByHeightFunc = func(ctx context.Context, height uint32) (*wdk.ChainBlockHeader, error)
	// GetStatusForTxIDsFunc is a function type for GetStatusForTxIDs service method.
	GetStatusForTxIDsFunc = func(ctx context.Context, txIDs []string) (*wdk.GetStatusForTxIDsResult, error)
	// GetUtxoStatusFunc is a function type for GetUtxoStatus service method.
	GetUtxoStatusFunc = func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (*wdk.UtxoStatusResult, error)
	// IsUtxo is a function type for IsUtxo service method.
	IsUtxo = func(ctx context.Context, scriptHash string, outpoint *transaction.Outpoint) (bool, error)
	// BsvExchangeRateFunc is a function type for BsvExchangeRate service method.
	BsvExchangeRateFunc = func(ctx context.Context) (float64, error)
)

// EFTransaction is one transaction of a PostEFBatch call.
type EFTransaction struct {
	// TxID is the transaction's id, as a hex string.
	TxID string
	// EF is the transaction in extended format (BRC-30), as bytes. It must not be modified.
	EF []byte
}

// Implementation defines all the methods that the services component supports.
// Each field corresponds to a specific service function.
// When a field is nil, it indicates that the particular service function doesn't have an implementation available - other services will be tried in that case.
type Implementation struct {
	RawTx                RawTxFunc
	PostTX               PostTXFunc
	PostEF               PostEFFunc
	PostEFBatch          PostEFBatchFunc
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

// ToImplementation creates an Implementation instance from the provided source with compatible method bindings.
// It scans the source object for methods matching the names and signatures of Implementation's fields.
// If a match is found and types are assignable, it sets the source method to the corresponding field in Implementation.
func ToImplementation(source any) Implementation {
	target := &Implementation{}
	sourceVal := reflect.ValueOf(source)
	targetVal := reflect.ValueOf(target).Elem()
	targetType := targetVal.Type()

	for i := range targetType.NumField() {
		field := targetType.Field(i)

		if field.Type.Kind() != reflect.Func {
			continue
		}

		method := sourceVal.MethodByName(field.Name)
		if method.IsValid() && method.Type().AssignableTo(field.Type) {
			targetVal.FieldByName(field.Name).Set(method)
		}
	}

	return *target
}
