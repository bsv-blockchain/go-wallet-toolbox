package mapping

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/slices"
)

// MapInternalizeActionArgs maps sdk.InternalizeActionArgs to wdk.InternalizeActionArgs
func MapInternalizeActionArgs(args sdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
	return wdk.InternalizeActionArgs{
		Tx:             primitives.ExplicitByteArray(args.Tx),
		Outputs:        slices.Map(args.Outputs, mapInternalizeOutput),
		Description:    primitives.String5to2000Bytes(args.Description),
		Labels:         slices.Map(args.Labels, stringToStringUnder300),
		SeekPermission: mapSeekPermission(args.SeekPermission),
	}
}

func stringToStringUnder300(s string) primitives.StringUnder300 {
	return primitives.StringUnder300(s)
}

// mapInternalizeOutput maps sdk.InternalizeOutput to wdk.InternalizeOutput
func mapInternalizeOutput(output sdk.InternalizeOutput) *wdk.InternalizeOutput {
	result := &wdk.InternalizeOutput{
		OutputIndex: output.OutputIndex,
		Protocol:    wdk.InternalizeProtocol(output.Protocol),
	}

	if output.PaymentRemittance != nil {
		result.PaymentRemittance = mapPaymentRemittance(output.PaymentRemittance)
	}

	if output.InsertionRemittance != nil {
		result.InsertionRemittance = mapInsertionRemittance(output.InsertionRemittance)
	}

	return result
}

// mapPaymentRemittance maps sdk.Payment to wdk.WalletPayment
func mapPaymentRemittance(payment *sdk.Payment) *wdk.WalletPayment {
	return &wdk.WalletPayment{
		DerivationPrefix:  primitives.Base64String(payment.DerivationPrefix),
		DerivationSuffix:  primitives.Base64String(payment.DerivationSuffix),
		SenderIdentityKey: primitives.PubKeyHex(payment.SenderIdentityKey),
	}
}

// mapInsertionRemittance maps sdk.BasketInsertion to wdk.BasketInsertion
func mapInsertionRemittance(insertion *sdk.BasketInsertion) *wdk.BasketInsertion {
	var customInstructions *string
	if insertion.CustomInstructions != "" {
		customInstructions = &insertion.CustomInstructions
	}

	return &wdk.BasketInsertion{
		Basket:             primitives.StringUnder300(insertion.Basket),
		CustomInstructions: customInstructions,
		Tags:               slices.Map(insertion.Tags, stringToStringUnder300),
	}
}

// mapSeekPermission maps *bool to *primitives.BooleanDefaultTrue
func mapSeekPermission(seekPermission *bool) *primitives.BooleanDefaultTrue {
	if seekPermission == nil {
		return nil
	}
	result := primitives.BooleanDefaultTrue(*seekPermission)
	return &result
}

// MapInternalizeActionResult maps *wdk.InternalizeActionResult to *sdk.InternalizeActionResult
func MapInternalizeActionResult(result *wdk.InternalizeActionResult) *sdk.InternalizeActionResult {
	return &sdk.InternalizeActionResult{
		Accepted: result.Accepted,
	}
}

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

	if args.Include == sdk.OutputIncludeEntireTransactions {
		result.IncludeTransactions = true
	} else if args.Include == sdk.OutputIncludeLockingScripts {
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
	sdkResult := &sdk.ListOutputsResult{
		TotalOutputs: uint32(result.TotalOutputs),
		Outputs:      slices.Map(result.Outputs, mapListOutputsOutput),
	}

	if result.BEEF != nil {
		sdkResult.BEEF = []byte(*result.BEEF)
	}

	return sdkResult
}
