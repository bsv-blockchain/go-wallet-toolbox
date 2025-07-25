package fixtures

import (
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

const (
	// DefaultAbortActionReference is a test reference for abort action
	DefaultAbortActionReference = "s7Tcy8M+5fLQ/XAk"
)

// DefaultWalletAbortActionArgs returns default SDK AbortActionArgs for testing
func DefaultWalletAbortActionArgs() sdk.AbortActionArgs {
	return sdk.AbortActionArgs{
		Reference: []byte(DefaultAbortActionReference),
	}
}

// DefaultWalletAbortActionArgsWithReference returns SDK AbortActionArgs with custom reference
func DefaultWalletAbortActionArgsWithReference(reference string) sdk.AbortActionArgs {
	return sdk.AbortActionArgs{
		Reference: []byte(reference),
	}
}
