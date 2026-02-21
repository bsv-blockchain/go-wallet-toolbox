package storage

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/spv"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// DefaultScriptsVerifier is the default implementation of the ScriptsVerifier interface for transaction validation without merkle path.
type DefaultScriptsVerifier struct{}

// NewDefaultScriptsVerifier creates a new instance of DefaultScriptsVerifier for ef transaction validation.
func NewDefaultScriptsVerifier() *DefaultScriptsVerifier {
	return &DefaultScriptsVerifier{}
}

// VerifyScripts verifies the given transaction by verify it's scripts.
// Returns true if valid or false with an error if invalid or verification fails.
func (b *DefaultScriptsVerifier) VerifyScripts(ctx context.Context, tx *transaction.Transaction) (bool, error) {
	if tx == nil {
		return false, fmt.Errorf("nil transaction")
	}

	ok, err := spv.VerifyScripts(ctx, tx)
	if err != nil {
		return false, fmt.Errorf("failed to verify scripts for tx: %s, err: %w", tx.TxID().String(), err)
	}

	return ok, nil
}
