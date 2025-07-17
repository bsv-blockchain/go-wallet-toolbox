package fixtures

import (
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

func DefaultWalletListOutputsArgs() sdk.ListOutputsArgs {
	return sdk.ListOutputsArgs{
		Basket:                    "",
		Tags:                      []string{},
		Limit:                     WalletPagingLimit,
		Offset:                    WalletPagingOffset,
		TagQueryMode:              sdk.QueryModeAny,
		IncludeCustomInstructions: nil,
		IncludeTags:               nil,
		IncludeLabels:             nil,
		SeekPermission:            nil,
	}
}
