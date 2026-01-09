package mapping

import (
	"fmt"
	"math"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

// MapListOutputsArgs maps sdk.ListOutputsArgs to wdk.ListOutputsArgs
func MapListOutputsArgs(args sdk.ListOutputsArgs) wdk.ListOutputsArgs {
	result := wdk.ListOutputsArgs{
		Basket:                    primitives.StringUnder300(args.Basket),
		Tags:                      slices.Map(args.Tags, func(tag string) primitives.StringUnder300 { return primitives.StringUnder300(tag) }),
		Limit:                     primitives.PositiveIntegerDefault10Max10000(to.ValueOr(args.Limit, 10)),
		Offset:                    primitives.PositiveInteger(to.ValueOr(args.Offset, 0)),
		IncludeCustomInstructions: optional.OfPtr(args.IncludeCustomInstructions).OrZeroValue(),
		IncludeTags:               optional.OfPtr(args.IncludeTags).OrZeroValue(),
		IncludeLabels:             optional.OfPtr(args.IncludeLabels).OrZeroValue(),
		SeekPermission:            optional.OfPtr(args.SeekPermission).OrZeroValue(),
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

	return result
}

// mapListOutputsOutput maps *wdk.WalletOutput to sdk.Output
func mapListOutputsOutput(output *wdk.WalletOutput) (sdk.Output, error) {
	result := sdk.Output{
		Satoshis:  uint64(output.Satoshis),
		Spendable: output.Spendable,
	}

	if output.Outpoint != "" {
		txID, vout, err := output.Outpoint.Get()
		if err != nil {
			return sdk.Output{}, fmt.Errorf("failed to get outpoint: %w", err)
		}

		txidHash, err := chainhash.NewHashFromHex(txID)
		if err != nil {
			return sdk.Output{}, fmt.Errorf("failed to parse transaction ID '%s': %w", txID, err)
		}

		result.Outpoint = transaction.Outpoint{
			Txid:  *txidHash,
			Index: vout,
		}
	}

	if output.CustomInstructions != nil {
		result.CustomInstructions = *output.CustomInstructions
	}

	lockingScript, err := parseLockingScript(output.LockingScript)
	if err != nil {
		return sdk.Output{}, fmt.Errorf("failed to parse locking script: %w", err)
	}
	result.LockingScript = lockingScript

	if len(output.Tags) > 0 {
		result.Tags = convertStringLikeSlice[string](output.Tags)
	}

	if len(output.Labels) > 0 {
		result.Labels = convertStringLikeSlice[string](output.Labels)
	}

	return result, nil
}

// MapListOutputsResult maps *wdk.ListOutputsResult to *sdk.ListOutputsResult
func MapListOutputsResult(result *wdk.ListOutputsResult) (*sdk.ListOutputsResult, error) {
	totalOutputs := min(uint64(result.TotalOutputs), math.MaxUint32)

	outputs, err := slices.MapOrError(result.Outputs, mapListOutputsOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to map outputs: %w", err)
	}

	totalOutputsUint32, err := to.UInt32(totalOutputs)
	if err != nil {
		return nil, fmt.Errorf("total outputs exceeds maximum allowed value: %w", err)
	}

	sdkResult := &sdk.ListOutputsResult{
		TotalOutputs: totalOutputsUint32,
		Outputs:      outputs,
	}

	sdkResult.BEEF = result.BEEF

	return sdkResult, nil
}

func convertStringLikeSlice[ResultType, ArgType ~string](input []ArgType) []ResultType {
	return slices.Map(input, func(s ArgType) ResultType { return ResultType(s) })
}

func parseLockingScript(hexPtr *primitives.HexString) ([]byte, error) {
	if hexPtr == nil {
		return nil, nil
	}
	lockingScript, err := script.NewFromHex(string(*hexPtr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse locking script from hex '%s': %w", string(*hexPtr), err)
	}
	return lockingScript.Bytes(), nil
}
