package wdk

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// Services defines an interface for handling 3rd party services
type Services interface {
	PostBEEF(ctx context.Context, beef *transaction.Beef, txids []string) (PostBeefResult, error)
}
