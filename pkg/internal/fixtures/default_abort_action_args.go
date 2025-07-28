package fixtures

import (
	"fmt"

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
func DefaultWalletAbortActionArgsWithReference[Ref string | []byte](reference Ref) sdk.AbortActionArgs {
	switch ref := any(reference).(type) {
	case string:
		return sdk.AbortActionArgs{
			Reference: []byte(ref),
		}
	case []byte:
		return sdk.AbortActionArgs{
			Reference: ref,
		}
	default:
		panic(fmt.Errorf("not supported reference type %T, check if all generics are handled", reference))
	}
}
