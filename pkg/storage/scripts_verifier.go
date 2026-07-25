package storage

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-sdk/script/interpreter"
	"github.com/bsv-blockchain/go-sdk/transaction"
)

// DefaultScriptsVerifier is the default implementation of the ScriptsVerifier interface for transaction validation without merkle path.
type DefaultScriptsVerifier struct{}

// NewDefaultScriptsVerifier creates a new instance of DefaultScriptsVerifier for ef transaction validation.
func NewDefaultScriptsVerifier() *DefaultScriptsVerifier {
	return &DefaultScriptsVerifier{}
}

// VerifyScripts executes the unlocking scripts of the given transaction
// against each input's source output — exactly what the interface contract
// promises: script execution only, no merkle path validation.
//
// It deliberately does NOT recurse into ancestors the way spv.VerifyScripts
// does. This verifier runs on the broadcast path for transactions built from
// the wallet's OWN outputs (createAction), where merkle proofs of our own
// ancestry are redundant work — proof checking belongs to internalizeAction,
// where someone else's transaction enters the wallet. The ancestor walk also
// re-computed every shared parent's BUMP root per spend (against a gullible
// header client, adding no trust) and dominated storage-server CPU at high
// TPS. Each input needs only its source output (script + satoshis).
func (b *DefaultScriptsVerifier) VerifyScripts(_ context.Context, tx *transaction.Transaction) (bool, error) {
	if tx == nil {
		return false, fmt.Errorf("nil transaction")
	}

	for vin, input := range tx.Inputs {
		sourceOutput := input.SourceTxOutput()
		if sourceOutput == nil {
			return false, fmt.Errorf("failed to verify scripts for tx: %s, err: missing source output: input %d", tx.TxID().String(), vin)
		}
		if err := interpreter.NewEngine().Execute(
			interpreter.WithTx(tx, vin, sourceOutput),
			interpreter.WithForkID(),
			interpreter.WithAfterGenesis(),
		); err != nil {
			return false, fmt.Errorf("failed to verify scripts for tx: %s, err: script verification failed: %w", tx.TxID().String(), err)
		}
	}

	return true, nil
}
