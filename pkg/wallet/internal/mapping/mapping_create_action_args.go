package mapping

import (
	"math"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/optional"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

// MapCreateActionArgs maps sdk.CreateActionArgs to wdk.ValidCreateActionArgs
func MapCreateActionArgs(args sdk.CreateActionArgs, opts wallet_opts.Flags) wdk.ValidCreateActionArgs {
	options := mapCreateActionOptions(to.Value(args.Options), opts)

	wdkArgs := &wdk.ValidCreateActionArgs{
		Description: primitives.String5to2000Bytes(args.Description),
		InputBEEF:   args.InputBEEF,
		Inputs:      []wdk.ValidCreateActionInput{},
		Outputs:     []wdk.ValidCreateActionOutput{},
		LockTime:    to.Value(args.LockTime),
		Version:     to.ValueOr(args.Version, 1),
		Labels:      []primitives.StringUnder300{},
		Options:     options,

		RandomVals:                   nil,
		IncludeAllSourceTransactions: opts.IncludeAllSourceTransactions,
	}

	if len(args.Inputs) > 0 {
		wdkArgs.Inputs = slices.Map(args.Inputs, mapCreateActionInput)
	}
	if len(args.Outputs) > 0 {
		wdkArgs.Outputs = slices.Map(args.Outputs, mapCreateActionOutput)
	}
	if len(args.Labels) > 0 {
		wdkArgs.Labels = slices.Map(args.Labels, stringToStringUnder300)
	}

	initComputableFields(args, wdkArgs)

	return *wdkArgs
}

func initComputableFields(args sdk.CreateActionArgs, wdkArgs *wdk.ValidCreateActionArgs) {
	wdkArgs.IsSendWith = len(wdkArgs.Options.SendWith) > 0
	wdkArgs.IsRemixChange = !wdkArgs.IsSendWith && len(wdkArgs.Inputs) == 0 && len(wdkArgs.Outputs) == 0
	wdkArgs.IsNewTx = wdkArgs.IsRemixChange || len(wdkArgs.Inputs) > 0 || len(wdkArgs.Outputs) > 0
	wdkArgs.IsSignAction = wdkArgs.IsNewTx && (!wdkArgs.Options.SignAndProcess.Value() || seq.Exists(seq.FromSlice(args.Inputs), withoutUnlockingScript))
	wdkArgs.IsDelayed = wdkArgs.Options.AcceptDelayedBroadcast.Value()
	wdkArgs.IsNoSend = wdkArgs.Options.NoSend.Value()
}

func withoutUnlockingScript(input sdk.CreateActionInput) bool {
	return input.UnlockingScript == nil
}

func mapCreateActionInput(input sdk.CreateActionInput) wdk.ValidCreateActionInput {
	var unlockingScriptLength *primitives.PositiveInteger
	if len(input.UnlockingScript) > 0 {
		unlockingScriptLength = to.Ptr(primitives.PositiveInteger(len(input.UnlockingScript)))
	} else if input.UnlockingScriptLength > 0 {
		length := primitives.PositiveInteger(input.UnlockingScriptLength)
		unlockingScriptLength = &length
	}

	return wdk.ValidCreateActionInput{
		Outpoint:         mapOutpoint(input.Outpoint),
		InputDescription: primitives.String5to2000Bytes(input.InputDescription),
		SequenceNumber:   primitives.PositiveInteger(to.ValueOr(input.SequenceNumber, math.MaxUint32)),
		// NOTICE: We don't want to send the unlocking script to the storage.
		UnlockingScript:       nil,
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

	result := wdk.ValidCreateActionOutput{
		LockingScript:      primitives.HexString(script.NewFromBytes(output.LockingScript).String()),
		Satoshis:           primitives.SatoshiValue(output.Satoshis),
		OutputDescription:  primitives.String5to2000Bytes(output.OutputDescription),
		Basket:             basket,
		CustomInstructions: customInstructions,
		Tags:               []primitives.StringUnder300{},
	}

	if len(output.Tags) > 0 {
		result.Tags = slices.Map(output.Tags, stringToStringUnder300)
	}

	return result
}

func mapCreateActionOptions(options sdk.CreateActionOptions, walletOpts wallet_opts.Flags) wdk.ValidCreateActionOptions {
	result := wdk.ValidCreateActionOptions{
		SignAndProcess:         (*primitives.BooleanDefaultTrue)(options.SignAndProcess),
		AcceptDelayedBroadcast: (*primitives.BooleanDefaultTrue)(options.AcceptDelayedBroadcast),
		TrustSelf:              to.IfThen(is.NotEmpty(options.TrustSelf), &options.TrustSelf).ElseThen(walletOpts.TrustSelf),
		KnownTxids:             []primitives.TXIDHexString{},
		ReturnTXIDOnly:         (*primitives.BooleanDefaultFalse)(options.ReturnTXIDOnly),
		NoSend:                 (*primitives.BooleanDefaultFalse)(options.NoSend),
		NoSendChange:           []wdk.OutPoint{},
		SendWith:               []primitives.TXIDHexString{},
		RandomizeOutputs:       optional.OfPtr(options.RandomizeOutputs).OrElse(true),
	}

	if len(options.KnownTxids) > 0 {
		result.KnownTxids = slices.Map(options.KnownTxids, chainHashToTXIDHexString)
	}
	if len(options.NoSendChange) > 0 {
		result.NoSendChange = slices.Map(options.NoSendChange, mapOutpoint)
	}
	if len(options.SendWith) > 0 {
		result.SendWith = slices.Map(options.SendWith, chainHashToTXIDHexString)
	}
	return result
}

func mapOutpoint(outpoint transaction.Outpoint) wdk.OutPoint {
	return wdk.OutPoint{
		TxID: outpoint.Txid.String(),
		Vout: outpoint.Index,
	}
}

func chainHashToTXIDHexString(hash chainhash.Hash) primitives.TXIDHexString {
	return primitives.TXIDHexString(hash.String())
}
