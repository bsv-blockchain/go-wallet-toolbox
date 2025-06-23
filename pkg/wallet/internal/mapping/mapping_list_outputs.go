package mapping

import (
	"math"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slices"
)

// MapListOutputsArgs maps sdk.ListOutputsArgs to wdk.ListOutputsArgs
func MapListOutputsArgs(args sdk.ListOutputsArgs) wdk.ListOutputsArgs {
	result := wdk.ListOutputsArgs{
		Basket: primitives.StringUnder300(args.Basket),
		Tags:   slices.Map(args.Tags, func(tag string) primitives.StringUnder300 { return primitives.StringUnder300(tag) }),
		Limit:  primitives.PositiveIntegerDefault10Max10000(args.Limit),
		Offset: primitives.PositiveInteger(args.Offset),
	}

	switch args.TagQueryMode {
	case sdk.QueryModeAll:
		result.TagQueryMode = "all"
	case sdk.QueryModeAny:
		result.TagQueryMode = "any"
	default:
		result.TagQueryMode = "any"
	}

	switch args.Include {
	case sdk.OutputIncludeEntireTransactions:
		result.IncludeTransactions = true
	case sdk.OutputIncludeLockingScripts:
		result.IncludeLockingScripts = true
	}

	if args.IncludeCustomInstructions != nil {
		result.IncludeCustomInstructions = *args.IncludeCustomInstructions
	}

	if args.IncludeTags != nil {
		result.IncludeTags = *args.IncludeTags
	}

	if args.IncludeLabels != nil {
		result.IncludeLabels = *args.IncludeLabels
	}

	if args.SeekPermission != nil {
		result.SeekPermission = *args.SeekPermission
	}

	return result
}

// mapListOutputsOutput maps *wdk.WalletOutput to sdk.Output
func mapListOutputsOutput(output *wdk.WalletOutput) sdk.Output {
	txID, vout := output.Outpoint.MustGet()

	txidHash, err := chainhash.NewHashFromHex(txID)
	if err != nil {
		txidHash = &chainhash.Hash{}
	}

	result := sdk.Output{
		Satoshis:  uint64(output.Satoshis),
		Spendable: output.Spendable,
		Outpoint: transaction.Outpoint{
			Txid:  *txidHash,
			Index: vout,
		},
	}

	if output.CustomInstructions != nil {
		result.CustomInstructions = *output.CustomInstructions
	}

	if output.LockingScript != nil {
		if lockingScript, err := script.NewFromHex(string(*output.LockingScript)); err == nil {
			result.LockingScript = lockingScript.Bytes()
		}
	}

	if len(output.Tags) > 0 {
		result.Tags = slices.Map(output.Tags, func(tag primitives.StringUnder300) string { return string(tag) })
	}

	if len(output.Labels) > 0 {
		result.Labels = slices.Map(output.Labels, func(label primitives.StringUnder300) string { return string(label) })
	}

	return result
}

// MapListOutputsResult maps *wdk.ListOutputsResult to *sdk.ListOutputsResult
func MapListOutputsResult(result *wdk.ListOutputsResult) *sdk.ListOutputsResult {
	totalOutputs := uint64(result.TotalOutputs)
	if totalOutputs > math.MaxUint32 {
		totalOutputs = math.MaxUint32
	}

	sdkResult := &sdk.ListOutputsResult{
		TotalOutputs: uint32(totalOutputs),
		Outputs:      slices.Map(result.Outputs, mapListOutputsOutput),
	}

	if result.BEEF != nil {
		sdkResult.BEEF = []byte(*result.BEEF)
	}

	return sdkResult
}
