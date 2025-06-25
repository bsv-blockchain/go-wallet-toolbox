package mapping

import (
	"fmt"
	"math"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

// MapListActionsArgs maps sdk.ListActionsArgs to wdk.ListActionsArgs
func MapListActionsArgs(args sdk.ListActionsArgs) wdk.ListActionsArgs {
	result := wdk.ListActionsArgs{
		Labels: slices.Map(args.Labels, func(label string) primitives.StringUnder300 { return primitives.StringUnder300(label) }),
		Limit:  primitives.PositiveIntegerDefault10Max10000(args.Limit),
		Offset: primitives.PositiveIntegerDefault10Max10000(args.Offset),
	}

	switch args.LabelQueryMode {
	case sdk.QueryModeAll:
		labelQueryMode := primitives.LabelQueryModeString("all")
		result.LabelQueryMode = &labelQueryMode
	case sdk.QueryModeAny:
		labelQueryMode := primitives.LabelQueryModeString("any")
		result.LabelQueryMode = &labelQueryMode
	default:
		labelQueryMode := primitives.LabelQueryModeString("any")
		result.LabelQueryMode = &labelQueryMode
	}

	if args.SeekPermission != nil {
		result.SeekPermissions = to.Ptr(primitives.BooleanDefaultTrue(*args.SeekPermission))
	}

	if args.IncludeInputs != nil {
		result.IncludeInputs = to.Ptr(primitives.BooleanDefaultFalse(*args.IncludeInputs))
	}

	if args.IncludeOutputs != nil {
		result.IncludeOutputs = to.Ptr(primitives.BooleanDefaultFalse(*args.IncludeOutputs))
	}

	if args.IncludeLabels != nil {
		result.IncludeLabels = to.Ptr(primitives.BooleanDefaultFalse(*args.IncludeLabels))
	}

	if args.IncludeInputSourceLockingScripts != nil {
		result.IncludeInputSourceLockingScripts = to.Ptr(primitives.BooleanDefaultFalse(*args.IncludeInputSourceLockingScripts))
	}

	if args.IncludeInputUnlockingScripts != nil {
		result.IncludeInputUnlockingScripts = to.Ptr(primitives.BooleanDefaultFalse(*args.IncludeInputUnlockingScripts))
	}

	if args.IncludeOutputLockingScripts != nil {
		result.IncludeOutputLockingScripts = to.Ptr(primitives.BooleanDefaultFalse(*args.IncludeOutputLockingScripts))
	}

	return result
}

// MapListActionsResult maps *wdk.ListActionsResult to *sdk.ListActionsResult
func MapListActionsResult(result *wdk.ListActionsResult) (*sdk.ListActionsResult, error) {
	actions, err := slices.MapOrError(result.Actions, mapListActionsAction)
	if err != nil {
		return nil, fmt.Errorf("failed to map actions: %w", err)
	}

	if result.TotalActions > math.MaxUint32 {
		return nil, fmt.Errorf("total actions count %d exceeds uint32 maximum", result.TotalActions)
	}

	return &sdk.ListActionsResult{
		TotalActions: uint32(result.TotalActions),
		Actions:      actions,
	}, nil
}

// mapListActionsAction maps wdk.WalletAction to sdk.Action
func mapListActionsAction(action wdk.WalletAction) (sdk.Action, error) {
	var txidHash chainhash.Hash

	hash, err := chainhash.NewHashFromHex(action.TxID)
	if err != nil {
		return sdk.Action{}, fmt.Errorf("failed to convert txid to hash: %w", err)
	}
	txidHash = *hash

	satoshis, err := to.UInt64(action.Satoshis)
	if err != nil {
		return sdk.Action{}, fmt.Errorf("failed to convert satoshis to uint64: %w", err)
	}

	status, err := mapActionStatus(action.Status)
	if err != nil {
		return sdk.Action{}, fmt.Errorf("failed to map action status: %w", err)
	}

	result := sdk.Action{
		Txid:        txidHash,
		Satoshis:    satoshis,
		Status:      status,
		IsOutgoing:  action.IsOutgoing,
		Description: action.Description,
		Labels:      action.Labels,
		Version:     action.Version,
		LockTime:    action.LockTime,
		Inputs:      slices.Map(action.Inputs, mapActionInput),
		Outputs:     slices.Map(action.Outputs, mapActionOutput),
	}

	return result, nil
}

// mapActionStatus maps string status to sdk.ActionStatus
func mapActionStatus(status string) (sdk.ActionStatus, error) {
	switch status {
	case "completed":
		return sdk.ActionStatusCompleted, nil
	case "unprocessed":
		return sdk.ActionStatusUnprocessed, nil
	case "sending":
		return sdk.ActionStatusSending, nil
	case "unproven":
		return sdk.ActionStatusUnproven, nil
	case "unsigned":
		return sdk.ActionStatusUnsigned, nil
	case "nosend":
		return sdk.ActionStatusNoSend, nil
	case "nonfinal":
		return sdk.ActionStatusNonFinal, nil
	default:
		return "", fmt.Errorf("unknown action status: %s", status)
	}
}

// scriptBytes converts a hex string to script bytes, returns nil if conversion fails
func scriptBytes(hexString string) []byte {
	if hexString == "" {
		return nil
	}
	if script, err := script.NewFromHex(hexString); err == nil {
		return script.Bytes()
	}
	return nil
}

// mapActionInput maps wdk.WalletActionInput to sdk.ActionInput
func mapActionInput(input wdk.WalletActionInput) sdk.ActionInput {
	result := sdk.ActionInput{
		SourceSatoshis:   input.SourceSatoshis,
		InputDescription: input.InputDescription,
		SequenceNumber:   input.SequenceNumber,
	}

	if input.SourceOutpoint != "" {
		if outpoint, err := transaction.OutpointFromString(input.SourceOutpoint); err == nil {
			result.SourceOutpoint = *outpoint
		}
	}

	result.SourceLockingScript = scriptBytes(input.SourceLockingScript)
	result.UnlockingScript = scriptBytes(input.UnlockingScript)

	return result
}

// mapActionOutput maps wdk.WalletActionOutput to sdk.ActionOutput
func mapActionOutput(output wdk.WalletActionOutput) sdk.ActionOutput {
	result := sdk.ActionOutput{
		Satoshis:           output.Satoshis,
		Spendable:          output.Spendable,
		CustomInstructions: output.CustomInstructions,
		Tags:               output.Tags,
		OutputIndex:        output.OutputIndex,
		OutputDescription:  output.OutputDescription,
		Basket:             output.Basket,
		LockingScript:      scriptBytes(output.LockingScript),
	}

	return result
}
