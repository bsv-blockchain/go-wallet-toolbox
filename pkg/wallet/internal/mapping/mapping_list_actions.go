package mapping

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

// MapListActionsArgs maps sdk.ListActionsArgs to wdk.ListActionsArgs
func MapListActionsArgs(args sdk.ListActionsArgs) wdk.ListActionsArgs {
	result := wdk.ListActionsArgs{
		Labels: slices.Map(args.Labels, func(label string) primitives.StringUnder300 { return primitives.StringUnder300(label) }),
		Limit:  primitives.PositiveIntegerDefault10Max10000(to.ValueOr(args.Limit, 10)),
		Offset: primitives.PositiveInteger(to.ValueOr(args.Offset, 0)),
	}

	switch args.LabelQueryMode {
	case sdk.QueryModeAll:
		labelQueryMode := defs.QueryModeAll
		result.LabelQueryMode = &labelQueryMode
	case sdk.QueryModeAny:
		labelQueryMode := defs.QueryModeAny
		result.LabelQueryMode = &labelQueryMode
	default:
		labelQueryMode := defs.QueryModeAny
		result.LabelQueryMode = &labelQueryMode
	}

	if args.SeekPermission != nil {
		result.SeekPermission = to.Ptr(primitives.BooleanDefaultTrue(*args.SeekPermission))
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

	if args.Reference != nil && *args.Reference != "" {
		result.Reference = args.Reference
	}

	return result
}

// MapListActionsResult maps *wdk.ListActionsResult to *sdk.ListActionsResult
func MapListActionsResult(result *wdk.ListActionsResult) (*sdk.ListActionsResult, error) {
	actions, err := slices.MapOrError(result.Actions, mapListActionsAction)
	if err != nil {
		return nil, fmt.Errorf("failed to map actions: %w", err)
	}

	totalActions, err := to.UInt32(result.TotalActions)
	if err != nil {
		return nil, fmt.Errorf("failed to convert total actions to uint32: %w", err)
	}

	return &sdk.ListActionsResult{
		TotalActions: totalActions,
		Actions:      actions,
	}, nil
}

// mapListActionsAction maps wdk.WalletAction to sdk.Action
func mapListActionsAction(action wdk.WalletAction) (sdk.Action, error) {
	hash, err := chainhash.NewHashFromHex(action.TxID)
	if err != nil {
		return sdk.Action{}, fmt.Errorf("failed to convert txid to hash: %w", err)
	}

	status, err := mapActionStatus(action.Status)
	if err != nil {
		return sdk.Action{}, fmt.Errorf("failed to map action status: %w", err)
	}

	inputs, err := slices.MapOrError(action.Inputs, mapActionInput)
	if err != nil {
		return sdk.Action{}, fmt.Errorf("failed to map action inputs: %w", err)
	}

	outputs, err := slices.MapOrError(action.Outputs, mapActionOutput)
	if err != nil {
		return sdk.Action{}, fmt.Errorf("failed to map action outputs: %w", err)
	}

	result := sdk.Action{
		Txid:        *hash,
		Satoshis:    action.Satoshis,
		Status:      status,
		IsOutgoing:  action.IsOutgoing,
		Description: action.Description,
		Labels:      action.Labels,
		Version:     action.Version,
		LockTime:    action.LockTime,
		Inputs:      inputs,
		Outputs:     outputs,
	}

	return result, nil
}

// TODO: Temporary constant - "failed" ActionStatus is missing in sdk.ActionStatus, adjust this once go-sdk is updated.
const ActionStatusFailed sdk.ActionStatus = "failed"

// mapActionStatus maps string status to sdk.ActionStatus
func mapActionStatus(status string) (sdk.ActionStatus, error) {
	switch status {
	case "completed":
		return sdk.ActionStatusCompleted, nil
	case "failed":
		return ActionStatusFailed, nil
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

// scriptBytes converts a hex string to script bytes, returns nil if hex string is empty
func scriptBytes(hexString string) ([]byte, error) {
	if hexString == "" {
		return nil, nil
	}
	script, err := script.NewFromHex(hexString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse script from hex: %w", err)
	}
	return script.Bytes(), nil
}

// mapActionInput maps wdk.WalletActionInput to sdk.ActionInput
func mapActionInput(input wdk.WalletActionInput) (sdk.ActionInput, error) {
	result := sdk.ActionInput{
		SourceSatoshis:   input.SourceSatoshis,
		InputDescription: input.InputDescription,
		SequenceNumber:   input.SequenceNumber,
	}

	if input.SourceOutpoint != "" {
		outpoint, err := transaction.OutpointFromString(input.SourceOutpoint)
		if err != nil {
			return sdk.ActionInput{}, fmt.Errorf("failed to parse source outpoint: %w", err)
		}
		result.SourceOutpoint = *outpoint
	}

	sourceLockingScript, err := scriptBytes(input.SourceLockingScript)
	if err != nil {
		return sdk.ActionInput{}, fmt.Errorf("failed to parse source locking script: %w", err)
	}
	result.SourceLockingScript = sourceLockingScript

	unlockingScript, err := scriptBytes(input.UnlockingScript)
	if err != nil {
		return sdk.ActionInput{}, fmt.Errorf("failed to parse unlocking script: %w", err)
	}
	result.UnlockingScript = unlockingScript

	return result, nil
}

// mapActionOutput maps wdk.WalletActionOutput to sdk.ActionOutput
func mapActionOutput(output wdk.WalletActionOutput) (sdk.ActionOutput, error) {
	lockingScript, err := scriptBytes(output.LockingScript)
	if err != nil {
		return sdk.ActionOutput{}, fmt.Errorf("failed to parse locking script: %w", err)
	}

	result := sdk.ActionOutput{
		Satoshis:           output.Satoshis,
		Spendable:          output.Spendable,
		CustomInstructions: output.CustomInstructions,
		Tags:               output.Tags,
		OutputIndex:        output.OutputIndex,
		OutputDescription:  output.OutputDescription,
		Basket:             output.Basket,
		LockingScript:      lockingScript,
	}

	return result, nil
}
