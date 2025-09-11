package services

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Named[T any] struct {
	Name string
	Item T
}

type RawTxFunc = func(ctx context.Context, txID string) (*wdk.RawTxResult, error)
type PostBEEFFunc = func(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error)

type Implementation struct {
	RawTx RawTxFunc
}
