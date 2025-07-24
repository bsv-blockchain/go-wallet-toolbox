package fixtures

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

const (
	// DefaultAbortActionReference is a test reference for abort action
	DefaultAbortActionReference = "dGVzdC1yZWZlcmVuY2UtYWJvcnQ="
)

// DefaultValidAbortActionArgs returns default valid AbortActionArgs for testing
func DefaultValidAbortActionArgs() wdk.AbortActionArgs {
	return wdk.AbortActionArgs{
		Reference: primitives.Base64String(DefaultAbortActionReference),
	}
}
