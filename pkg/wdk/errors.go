package wdk

import (
	"errors"
	"fmt"
)

// ErrNotFoundError represents an error indicating that a requested resource or item was not found.
var ErrNotFoundError = fmt.Errorf("not found")

// ErrNotEnoughFunds is returned when a transaction cannot be funded due to insufficient UTXOs.
var ErrNotEnoughFunds = errors.New("not enough funds")

// ErrUTXOContention is returned when a concurrent transaction reserved one or more of the
// UTXOs this transaction selected, before this transaction could reserve them itself. Callers
// may retry the operation with a fresh UTXO selection.
var ErrUTXOContention = errors.New("utxo contention: concurrent transaction reserved one or more selected UTXOs")
