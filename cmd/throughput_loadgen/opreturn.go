package main

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// ProofPayload is the fixed OP_RETURN message for live throughput tests.
const ProofPayload = "the proof is in the pudding"

// OpReturnLockingScript builds a standard OP_RETURN locking script for payload.
func OpReturnLockingScript(payload string) ([]byte, error) {
	if payload == "" {
		return nil, fmt.Errorf("opreturn payload must be non-empty")
	}
	out, err := transaction.CreateOpReturnOutput([][]byte{[]byte(payload)})
	if err != nil {
		return nil, fmt.Errorf("create opreturn output: %w", err)
	}
	return out.LockingScript.Bytes(), nil
}
