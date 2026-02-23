package wdk

import (
	"context"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// ScriptsVerifier is an interface for verifying transaction scripts without merkle path validation.
type ScriptsVerifier interface {
	// VerifyScripts verifies the scripts of the given transaction.
	// Returns true if all scripts are valid, false otherwise.
	// This does NOT verify merkle paths - only script execution.
	VerifyScripts(ctx context.Context, tx *transaction.Transaction) (bool, error)
}
