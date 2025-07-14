package fixtures

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
)

const (
	// DefaultAbortActionReference is a test reference for abort action
	DefaultAbortActionReference = "test-reference-abort"

	// DefaultAbortActionTxID is a test txid for abort action (64 chars)
	DefaultAbortActionTxID = "756754d5ad8f00e05c36d89a852971c0a1dc0c10f20cd7840ead347aff475ef6"

	// ShortReference is a reference that's too short to be a txid
	ShortReference = "short-ref"
)

// DefaultValidAbortActionArgs returns default valid AbortActionArgs for testing
func DefaultValidAbortActionArgs() wdk.AbortActionArgs {
	return wdk.AbortActionArgs{
		Reference: to.Ptr(DefaultAbortActionReference),
	}
}

// DefaultValidAbortActionArgsWithTxID returns AbortActionArgs with a txid-like reference
func DefaultValidAbortActionArgsWithTxID() wdk.AbortActionArgs {
	return wdk.AbortActionArgs{
		Reference: to.Ptr(DefaultAbortActionTxID),
	}
}

// DefaultValidAbortActionArgsWithShortRef returns AbortActionArgs with a short reference
func DefaultValidAbortActionArgsWithShortRef() wdk.AbortActionArgs {
	return wdk.AbortActionArgs{
		Reference: to.Ptr(ShortReference),
	}
}

// DefaultAbortActionResult returns default AbortActionResult for testing
func DefaultAbortActionResult() *wdk.AbortActionResult {
	return &wdk.AbortActionResult{
		Aborted: true,
	}
}
