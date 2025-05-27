package mapping

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
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
