package mapping

import (
	"encoding/base64"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
)

// MapInternalizeActionArgs maps sdk.InternalizeActionArgs to wdk.InternalizeActionArgs
func MapInternalizeActionArgs(args sdk.InternalizeActionArgs) wdk.InternalizeActionArgs {
	return wdk.InternalizeActionArgs{
		Tx:             args.Tx,
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
	var senderIdentityKey primitives.PubKeyHex
	if payment.SenderIdentityKey != nil {
		senderIdentityKey = primitives.PubKeyHex(payment.SenderIdentityKey.ToDERHex())
	}

	return &wdk.WalletPayment{
		DerivationPrefix:  mapToBase64(payment.DerivationPrefix),
		DerivationSuffix:  mapToBase64(payment.DerivationSuffix),
		SenderIdentityKey: senderIdentityKey,
	}
}

func mapToBase64(bytes []byte) primitives.Base64String {
	result := base64.StdEncoding.EncodeToString(bytes)
	return primitives.Base64String(result)
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
