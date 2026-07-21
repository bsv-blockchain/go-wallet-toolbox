// Package stream implements the controllable createAction event stream.
package stream

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// HashPayload returns sha256(iteration string concatenated with RFC3339Nano timestamp).
func HashPayload(iteration uint64, ts time.Time) []byte {
	// Concatenate as decimal iteration + timestamp string (no separator required by the demo).
	msg := strconv.FormatUint(iteration, 10) + ts.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(msg))
	return sum[:]
}

// OpReturnLockingScriptForHash builds an OP_RETURN locking script that pushes the 32-byte hash.
func OpReturnLockingScriptForHash(hash []byte) ([]byte, error) {
	if len(hash) == 0 {
		return nil, fmt.Errorf("opreturn hash must be non-empty")
	}
	out, err := transaction.CreateOpReturnOutput([][]byte{hash})
	if err != nil {
		return nil, fmt.Errorf("create opreturn output: %w", err)
	}
	return out.LockingScript.Bytes(), nil
}

// OpReturnLockingScriptForIteration builds the OP_RETURN locking script for one stream iteration.
func OpReturnLockingScriptForIteration(iteration uint64, ts time.Time) ([]byte, error) {
	return OpReturnLockingScriptForHash(HashPayload(iteration, ts))
}
