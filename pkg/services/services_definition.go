package services

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type NamedFunc[F any] struct {
	Name string
	Func F
}

type RawTxFunc = func(ctx context.Context, txID string) (*wdk.RawTxResult, error)
type PostBEEFFunc = func(ctx context.Context, beef *transaction.Beef, txIDs []string) (*wdk.PostedBEEF, error)

type AllServicesDefinition struct {
	RawTx RawTxFunc
}

type allServicesDefinitionItem struct {
	AllServicesDefinition
	Name     string
	Priority int
}
