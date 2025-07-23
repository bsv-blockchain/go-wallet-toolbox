package fixtures

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	// DefaultAbortActionReference is a test reference for abort action
	DefaultAbortActionReference = "test-reference-abort"
)

// DefaultValidAbortActionArgs returns default valid AbortActionArgs for testing
func DefaultValidAbortActionArgs() wdk.AbortActionArgs {
	return wdk.AbortActionArgs{
		Reference: DefaultAbortActionReference,
	}
}
