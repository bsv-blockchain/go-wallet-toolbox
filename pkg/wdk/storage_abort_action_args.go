package wdk

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// AbortActionArgs defines the arguments for aborting a wallet action.
type AbortActionArgs struct {
	// Reference is the unique identifier for the action to be aborted.
	Reference primitives.Base64String `json:"reference"`
}

// AbortActionResult defines the result of an abort action operation.
type AbortActionResult struct {
	// Aborted indicates whether the action was successfully aborted.
	Aborted bool
}

// ErrNotAbortableAction indicates that the action cannot be aborted due to its current status or type.
var ErrNotAbortableAction = fmt.Errorf("action cannot be aborted")
