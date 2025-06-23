package mapping

import (
	"strconv"
	"strings"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slices"
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
		seekPermission := primitives.BooleanDefaultTrue(*args.SeekPermission)
		result.SeekPermissions = &seekPermission
	}
	if args.IncludeInputs != nil {
		includeInputs := primitives.BooleanDefaultFalse(*args.IncludeInputs)
		result.IncludeInputs = &includeInputs
	}
	if args.IncludeOutputs != nil {
		includeOutputs := primitives.BooleanDefaultFalse(*args.IncludeOutputs)
		result.IncludeOutputs = &includeOutputs
	}
	if args.IncludeLabels != nil {
		includeLabels := primitives.BooleanDefaultFalse(*args.IncludeLabels)
		result.IncludeLabels = &includeLabels
	}
	if args.IncludeInputSourceLockingScripts != nil {
		includeInputSourceLockingScripts := primitives.BooleanDefaultFalse(*args.IncludeInputSourceLockingScripts)
		result.IncludeInputSourceLockingScripts = &includeInputSourceLockingScripts
	}
	if args.IncludeInputUnlockingScripts != nil {
		includeInputUnlockingScripts := primitives.BooleanDefaultFalse(*args.IncludeInputUnlockingScripts)
		result.IncludeInputUnlockingScripts = &includeInputUnlockingScripts
	}
	if args.IncludeOutputLockingScripts != nil {
		includeOutputLockingScripts := primitives.BooleanDefaultFalse(*args.IncludeOutputLockingScripts)
		result.IncludeOutputLockingScripts = &includeOutputLockingScripts
	}

	return result
}

// MapListActionsResult maps *wdk.ListActionsResult to *sdk.ListActionsResult
func MapListActionsResult(result *wdk.ListActionsResult) *sdk.ListActionsResult {
	return &sdk.ListActionsResult{
		TotalActions: result.TotalActions,
		Actions:      slices.Map(result.Actions, mapListActionsAction),
	}
}

// mapListActionsAction maps wdk.WalletAction to sdk.Action
func mapListActionsAction(action wdk.WalletAction) sdk.Action {
	var txidHash chainhash.Hash
	if action.TxID != "" {
		if hash, err := chainhash.NewHashFromHex(action.TxID); err == nil {
			txidHash = *hash
		}
	}

	result := sdk.Action{
		Txid:        txidHash,
		Satoshis:    uint64(action.Satoshis),
		Status:      mapActionStatus(action.Status),
		IsOutgoing:  action.IsOutgoing,
		Description: action.Description,
		Labels:      action.Labels,
		Version:     action.Version,
		LockTime:    action.LockTime,
		Inputs:      slices.Map(action.Inputs, mapActionInput),
		Outputs:     slices.Map(action.Outputs, mapActionOutput),
	}

	return result
}

// mapActionStatus maps string status to sdk.ActionStatus
func mapActionStatus(status string) sdk.ActionStatus {
	switch status {
	case "completed":
		return sdk.ActionStatusCompleted
	case "unprocessed":
		return sdk.ActionStatusUnprocessed
	case "sending":
		return sdk.ActionStatusSending
	case "unproven":
		return sdk.ActionStatusUnproven
	case "unsigned":
		return sdk.ActionStatusUnsigned
	case "nosend":
		return sdk.ActionStatusNoSend
	case "nonfinal":
		return sdk.ActionStatusNonFinal
	default:
		return sdk.ActionStatusUnprocessed
	}
}

// mapActionInput maps wdk.WalletActionInput to sdk.ActionInput
func mapActionInput(input wdk.WalletActionInput) sdk.ActionInput {
	result := sdk.ActionInput{
		SourceSatoshis:   input.SourceSatoshis,
		InputDescription: input.InputDescription,
		SequenceNumber:   input.SequenceNumber,
	}

	parts := strings.Split(input.SourceOutpoint, ":")
	if len(parts) == 2 {
		if txidHash, err := chainhash.NewHashFromHex(parts[0]); err == nil {
			if vout, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
				result.SourceOutpoint = transaction.Outpoint{
					Txid:  *txidHash,
					Index: uint32(vout),
				}
			}
		}
	}

	if input.SourceLockingScript != "" {
		if lockingScript, err := script.NewFromHex(input.SourceLockingScript); err == nil {
			result.SourceLockingScript = lockingScript.Bytes()
		}
	}

	if input.UnlockingScript != "" {
		if unlockingScript, err := script.NewFromHex(input.UnlockingScript); err == nil {
			result.UnlockingScript = unlockingScript.Bytes()
		}
	}

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
	}

	if output.LockingScript != "" {
		if lockingScript, err := script.NewFromHex(output.LockingScript); err == nil {
			result.LockingScript = lockingScript.Bytes()
		}
	}

	return result
}
