package mapping

import (
	"strconv"
	"strings"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

// MapCreateActionArgs maps sdk.CreateActionArgs to wdk.ValidCreateActionArgs
func MapCreateActionArgs(args sdk.CreateActionArgs, opts wallet_opts.Opts) wdk.ValidCreateActionArgs {
	wdkArgs := &wdk.ValidCreateActionArgs{
		Description: primitives.String5to2000Bytes(args.Description),
		InputBEEF:   args.InputBEEF,
		Inputs:      slices.Map(args.Inputs, mapCreateActionInput),
		Outputs:     slices.Map(args.Outputs, mapCreateActionOutput),
		LockTime:    args.LockTime,
		Version:     args.Version,
		Labels:      slices.Map(args.Labels, stringToStringUnder300),
		Options:     mapCreateActionOptions(optional.OfPtr(args.Options).OrZeroValue(), opts),

		RandomVals:                   nil,
		IncludeAllSourceTransactions: opts.IncludeAllSourceTransactions,
	}

	initComputableFields(wdkArgs)

	return *wdkArgs
}

func initComputableFields(wdkArgs *wdk.ValidCreateActionArgs) {
	wdkArgs.IsSendWith = len(wdkArgs.Options.SendWith) > 0
	wdkArgs.IsRemixChange = !wdkArgs.IsSendWith && len(wdkArgs.Inputs) == 0 && len(wdkArgs.Outputs) == 0
	wdkArgs.IsNewTx = wdkArgs.IsRemixChange || len(wdkArgs.Inputs) > 0 || len(wdkArgs.Outputs) > 0
	wdkArgs.IsSignAction = wdkArgs.IsNewTx && (!wdkArgs.Options.SignAndProcess.Value() || seq.Exists(seq.FromSlice(wdkArgs.Inputs), withoutUnlockingScript))
	wdkArgs.IsDelayed = wdkArgs.Options.AcceptDelayedBroadcast.Value()
	wdkArgs.IsNoSend = wdkArgs.Options.NoSend.Value()
}

func withoutUnlockingScript(input wdk.ValidCreateActionInput) bool {
	return input.UnlockingScript != nil
}

func mapCreateActionInput(input sdk.CreateActionInput) wdk.ValidCreateActionInput {
	outpoint := parseOutpoint(input.Outpoint)

	var unlockingScript *primitives.HexString
	if input.UnlockingScript != "" {
		script := primitives.HexString(input.UnlockingScript)
		unlockingScript = &script
	}

	var unlockingScriptLength *primitives.PositiveInteger
	if input.UnlockingScriptLength > 0 {
		length := primitives.PositiveInteger(input.UnlockingScriptLength)
		unlockingScriptLength = &length
	}

	return wdk.ValidCreateActionInput{
		Outpoint:              outpoint,
		InputDescription:      primitives.String5to2000Bytes(input.InputDescription),
		SequenceNumber:        primitives.PositiveInteger(input.SequenceNumber),
		UnlockingScript:       unlockingScript,
		UnlockingScriptLength: unlockingScriptLength,
	}
}

func mapCreateActionOutput(output sdk.CreateActionOutput) wdk.ValidCreateActionOutput {
	var basket *primitives.StringUnder300
	if output.Basket != "" {
		b := primitives.StringUnder300(output.Basket)
		basket = &b
	}

	var customInstructions *string
	if output.CustomInstructions != "" {
		customInstructions = &output.CustomInstructions
	}

	return wdk.ValidCreateActionOutput{
		LockingScript:      primitives.HexString(output.LockingScript),
		Satoshis:           primitives.SatoshiValue(output.Satoshis),
		OutputDescription:  primitives.String5to2000Bytes(output.OutputDescription),
		Basket:             basket,
		CustomInstructions: customInstructions,
		Tags:               slices.Map(output.Tags, stringToStringUnder300),
	}
}

func mapCreateActionOptions(options sdk.CreateActionOptions, walletOpts wallet_opts.Opts) wdk.ValidCreateActionOptions {
	return wdk.ValidCreateActionOptions{
		SignAndProcess:         (*primitives.BooleanDefaultTrue)(options.SignAndProcess),
		AcceptDelayedBroadcast: (*primitives.BooleanDefaultTrue)(options.AcceptDelayedBroadcast),
		TrustSelf:              to.IfThen(is.NotEmpty(options.TrustSelf), &options.TrustSelf).ElseThen(walletOpts.TrustSelf),
		KnownTxids:             slices.Map(options.KnownTxids, stringToTXIDHexString),
		ReturnTXIDOnly:         (*primitives.BooleanDefaultFalse)(options.ReturnTXIDOnly),
		NoSend:                 (*primitives.BooleanDefaultFalse)(options.NoSend),
		NoSendChange:           slices.Map(options.NoSendChange, parseOutpoint),
		SendWith:               slices.Map(options.SendWith, stringToTXIDHexString),
		RandomizeOutputs:       optional.OfPtr(options.RandomizeOutputs).OrElse(true),
	}
}

func parseOutpoint(outpoint string) wdk.OutPoint {
	parts := strings.Split(outpoint, ".")
	if len(parts) != 2 {
		return wdk.OutPoint{}
	}

	voutUint64, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return wdk.OutPoint{}
	}

	return wdk.OutPoint{
		TxID: parts[0],
		Vout: uint32(voutUint64),
	}
}

func stringToTXIDHexString(s string) primitives.TXIDHexString {
	return primitives.TXIDHexString(s)
}
