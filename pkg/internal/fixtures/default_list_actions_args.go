package fixtures

import (
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
)

// DefaultWalletListActionsArgs returns default SDK ListActionsArgs for testing
func DefaultWalletListActionsArgs() sdk.ListActionsArgs {
	return sdk.ListActionsArgs{
		Labels:                           nil,
		Limit:                            WalletPagingLimit,
		Offset:                           WalletPagingOffset,
		LabelQueryMode:                   sdk.QueryModeAny,
		SeekPermission:                   to.Ptr(true),
		IncludeInputs:                    to.Ptr(false),
		IncludeOutputs:                   to.Ptr(false),
		IncludeLabels:                    to.Ptr(false),
		IncludeInputSourceLockingScripts: to.Ptr(false),
		IncludeInputUnlockingScripts:     to.Ptr(false),
		IncludeOutputLockingScripts:      to.Ptr(false),
	}
}

// DefaultWalletListActionsArgsWithIncludes returns SDK ListActionsArgs with all includes enabled
func DefaultWalletListActionsArgsWithIncludes() sdk.ListActionsArgs {
	return sdk.ListActionsArgs{
		Labels:                           nil,
		Limit:                            to.Ptr[uint32](10),
		Offset:                           WalletPagingOffset,
		LabelQueryMode:                   sdk.QueryModeAny,
		SeekPermission:                   to.Ptr(true),
		IncludeInputs:                    to.Ptr(true),
		IncludeOutputs:                   to.Ptr(true),
		IncludeLabels:                    to.Ptr(true),
		IncludeInputSourceLockingScripts: to.Ptr(true),
		IncludeInputUnlockingScripts:     to.Ptr(true),
		IncludeOutputLockingScripts:      to.Ptr(true),
	}
}
