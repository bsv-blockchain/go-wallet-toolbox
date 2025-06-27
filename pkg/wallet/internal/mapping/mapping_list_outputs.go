package mapping

import (
	"fmt"
	"math"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
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
		result.TagQueryMode = to.Ptr(defs.QueryModeAll)
	case sdk.QueryModeAny:
		result.TagQueryMode = to.Ptr(defs.QueryModeAny)
	default:
		result.TagQueryMode = to.Ptr(defs.QueryModeAny)
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
func mapListOutputsOutput(output *wdk.WalletOutput) (sdk.Output, error) {
	txID, vout := output.Outpoint.MustGet()

	txidHash, err := chainhash.NewHashFromHex(txID)
	if err != nil {
		return sdk.Output{}, fmt.Errorf("failed to parse transaction ID '%s': %w", txID, err)
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

	result.LockingScript = parseLockingScript(output.LockingScript)

	if len(output.Tags) > 0 {
		result.Tags = mapStrings(output.Tags)
	}

	if len(output.Labels) > 0 {
		result.Labels = mapStrings(output.Labels)
	}

	return result, nil
}

// MapListOutputsResult maps *wdk.ListOutputsResult to *sdk.ListOutputsResult
func MapListOutputsResult(result *wdk.ListOutputsResult) (*sdk.ListOutputsResult, error) {
	totalOutputs := min(uint64(result.TotalOutputs), math.MaxUint32)

	outputs := make([]sdk.Output, 0, len(result.Outputs))
	for _, output := range result.Outputs {
		mappedOutput, err := mapListOutputsOutput(output)
		if err != nil {
			return nil, fmt.Errorf("failed to map output: %w", err)
		}
		outputs = append(outputs, mappedOutput)
	}

	totalOutputsUint32, err := to.UInt32(totalOutputs)
	if err != nil {
		return nil, fmt.Errorf("total outputs exceeds maximum allowed value: %w", err)
	}

	sdkResult := &sdk.ListOutputsResult{
		TotalOutputs: totalOutputsUint32,
		Outputs:      outputs,
	}

	if result.BEEF != nil {
		sdkResult.BEEF = []byte(*result.BEEF)
	}

	return sdkResult, nil
}

func mapStrings(input []primitives.StringUnder300) []string {
	return slices.Map(input, func(s primitives.StringUnder300) string { return string(s) })
}

func parseLockingScript(hexPtr *primitives.HexString) []byte {
	if hexPtr == nil {
		return nil
	}
	lockingScript, err := script.NewFromHex(string(*hexPtr))
	if err != nil {
		return nil
	}
	return lockingScript.Bytes()
}
