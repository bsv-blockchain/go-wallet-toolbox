package validate

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/is"
)

func ValidCreateActionArgs(args *wdk.ValidCreateActionArgs) error {
	err := WalletCreateActionArgs(args)
	if err != nil {
		return err
	}

	deducedIsSendWith := len(args.Options.SendWith) > 0
	if args.IsSendWith != deducedIsSendWith {
		return fmt.Errorf("inconsistent IsSendWith with Options.SendWith")
	}

	deducedIsRemixChange := !args.IsSendWith && len(args.Inputs) == 0 && len(args.Outputs) == 0
	if args.IsRemixChange != deducedIsRemixChange {
		return fmt.Errorf("inconsistent IsRemixChange with IsSendWith and Inputs and Outputs")
	}

	deducedIsNewTx := args.IsRemixChange || len(args.Inputs) > 0 || len(args.Outputs) > 0
	if args.IsNewTx != deducedIsNewTx {
		return fmt.Errorf("inconsistent IsNewTx with IsRemixChange and Inputs and Outputs")
	}

	if !args.IsNewTx {
		return fmt.Errorf("create action is meant to create a new transaction")
	}

	// NOTICE: because wallet is removing unlocking scripts before sending,
	// therefore we can only check the situation, when IsNewTx and SignAndProcess are true but IsSignAction is false
	deducedIsSignAction := args.IsNewTx && !args.Options.SignAndProcess.Value()
	if deducedIsSignAction && !args.IsSignAction {
		return fmt.Errorf("inconsistent IsSignAction (%v) with IsNewTx (%v) and Options.SignAndProcess (%v)", args.IsSignAction, args.IsNewTx, args.Options.SignAndProcess.Value())
	}

	deducedIsDelayed := args.Options.AcceptDelayedBroadcast.Value()
	if args.IsDelayed != deducedIsDelayed {
		return fmt.Errorf("inconsistent IsDelayed with Options.AcceptDelayedBroadcast")
	}

	deducedIsNoSend := args.Options.NoSend.Value()
	if args.IsNoSend != deducedIsNoSend {
		return fmt.Errorf("inconsistent IsNoSend with Options.NoSend")
	}

	return nil
}

func WalletCreateActionArgs(args *wdk.ValidCreateActionArgs) error {
	if err := args.Description.Validate(); err != nil {
		return fmt.Errorf("the description parameter must be %w", err)
	}

	if len(args.Reference) > 500 {
		return fmt.Errorf("reference must be 500 characters or less")
	}

	for i, label := range args.Labels {
		if err := label.Validate(); err != nil {
			return fmt.Errorf("label as %d must be %w", i, err)
		}
	}

	seenInputs := make(map[wdk.OutPoint]struct{})
	for i, input := range args.Inputs {
		if err := primitives.TXIDHexString(input.Outpoint.TxID).Validate(); err != nil {
			return fmt.Errorf("txid from outpoint in input %d is invalid: %w", i, err)
		}
		if _, exists := seenInputs[input.Outpoint]; exists {
			return fmt.Errorf("duplicate input outpoint at index %d: %s.%d", i, input.Outpoint.TxID, input.Outpoint.Vout)
		}
		if err := validateCreateActionInput(&input); err != nil {
			return fmt.Errorf("invalid input as %d: %w", i, err)
		}
		seenInputs[input.Outpoint] = struct{}{}
	}

	for i, output := range args.Outputs {
		if err := validateCreateActionOutput(&output); err != nil {
			return fmt.Errorf("invalid output at index %d: %w", i, err)
		}
	}

	if !args.IsNewTx && len(args.Options.SendWith) == 0 {
		return fmt.Errorf("IsNewTx is false but no sendWith arguments provided")
	}

	if !args.IsNoSend && len(args.Options.NoSendChange) > 0 {
		return fmt.Errorf("NoSendChange outputs are supported only with IsNoSend flag")
	}

	if args.IsNoSend && len(args.Options.NoSendChange) > 0 && !is.UniqueSlice(args.Options.NoSendChange) {
		return fmt.Errorf("duplicated outpoints in Options.NoSendChange")
	}

	return nil
}

func validateCreateActionInput(input *wdk.ValidCreateActionInput) error {
	if input.UnlockingScript == nil && input.UnlockingScriptLength == nil {
		return fmt.Errorf("at least one of unlockingScript, unlockingScriptLength must be set")
	}

	if input.UnlockingScript != nil {
		if err := input.UnlockingScript.Validate(); err != nil {
			return fmt.Errorf("unlockingScript must be %w", err)
		}

		if input.UnlockingScriptLength != nil && uint(len(*input.UnlockingScript)) != uint(*input.UnlockingScriptLength) {
			return fmt.Errorf("unlockingScriptLength must match provided unlockingScript length")
		}
	}

	if err := input.InputDescription.Validate(); err != nil {
		return fmt.Errorf("inputDescription must be %w", err)
	}

	return nil
}

func validateCreateActionOutput(output *wdk.ValidCreateActionOutput) error {
	if err := output.LockingScript.Validate(); err != nil {
		return fmt.Errorf("lockingScript must be %w", err)
	}

	if err := output.Satoshis.Validate(); err != nil {
		return fmt.Errorf("satoshis must be %w", err)
	}

	if err := output.OutputDescription.Validate(); err != nil {
		return fmt.Errorf("outputDescription must be %w", err)
	}

	if output.Basket != nil {
		if err := output.Basket.Validate(); err != nil {
			return fmt.Errorf("basket must be %w", err)
		}
	}

	for _, tag := range output.Tags {
		if err := tag.Validate(); err != nil {
			return fmt.Errorf("tag must be %w", err)
		}
	}

	return nil
}
