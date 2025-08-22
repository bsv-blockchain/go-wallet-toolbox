package storage

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
)

type DefaultBeefVerifier struct{}

func (b *DefaultBeefVerifier) VerifyBeef(ctx context.Context, beef *transaction.Beef, chainTracker chaintracker.ChainTracker, allowTxidOnly bool) (bool, error) {
	if beef == nil {
		return false, fmt.Errorf("nil beef")
	}
	return beef.Verify(ctx, chainTracker, allowTxidOnly)
}



