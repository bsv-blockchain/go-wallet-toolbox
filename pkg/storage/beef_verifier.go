package storage

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
)

// DefaultBeefVerifier is the default implementation of the BeefVerifier interface for beef transaction validation.
type DefaultBeefVerifier struct{}

// VerifyBeef verifies the given Beef transaction using the provided chain tracker and verification mode.
// Returns true if valid or false with an error if invalid or verification fails.
func (b *DefaultBeefVerifier) VerifyBeef(ctx context.Context, beef *transaction.Beef, chainTracker chaintracker.ChainTracker, allowTxidOnly bool) (bool, error) {
	if beef == nil {
		return false, fmt.Errorf("nil beef")
	}

	ok, err := beef.Verify(ctx, chainTracker, allowTxidOnly)
	if err != nil {
		return false, fmt.Errorf("beef verification failed: %w", err)
	}

	return ok, nil
}
