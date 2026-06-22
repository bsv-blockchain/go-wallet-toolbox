package wdk

import (
	"errors"
	"fmt"
)

// ErrNotFoundError represents an error indicating that a requested resource or item was not found.
var ErrNotFoundError = fmt.Errorf("not found")

// ErrNotEnoughFunds is returned when a transaction cannot be funded due to insufficient UTXOs.
var ErrNotEnoughFunds = errors.New("not enough funds")
