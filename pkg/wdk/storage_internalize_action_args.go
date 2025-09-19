package wdk

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TODO: Check if the types below can be replaced by go-sdk types

// InternalizeProtocol is the protocol used for internalizing the action
type InternalizeProtocol string

// Possible values for InternalizeProtocol
const (
	WalletPaymentProtocol   InternalizeProtocol = "wallet payment"
	BasketInsertionProtocol InternalizeProtocol = "basket insertion"
)

// WalletPayment represents the payment remittance for the "wallet payment" protocol
type WalletPayment struct {
	DerivationPrefix  primitives.Base64String `json:"derivationPrefix"`
	DerivationSuffix  primitives.Base64String `json:"derivationSuffix"`
	SenderIdentityKey primitives.PubKeyHex    `json:"senderIdentityKey"`
}

// Validate checks if the WalletPayment is valid
func (w *WalletPayment) Validate() error {
	if err := w.DerivationPrefix.Validate(); err != nil {
		return fmt.Errorf("derivation prefix must be %w", err)
	}
	if err := w.DerivationSuffix.Validate(); err != nil {
		return fmt.Errorf("derivation suffix must be %w", err)
	}
	if err := w.SenderIdentityKey.Validate(); err != nil {
		return fmt.Errorf("sender identity key must be %w", err)
	}
	return nil
}

// BasketInsertion represents the insertion remittance for the "basket insertion" protocol
type BasketInsertion struct {
	Basket             primitives.StringUnder300   `json:"basket"`
	CustomInstructions *string                     `json:"customInstructions"`
	Tags               []primitives.StringUnder300 `json:"tags"`
}

// Validate checks if the BasketInsertion is valid
func (b *BasketInsertion) Validate() error {
	if b.Basket == "" {
		return fmt.Errorf("basket cannot be empty")
	}
	if err := b.Basket.Validate(); err != nil {
		return fmt.Errorf("basket must be %w", err)
	}
	for i, tag := range b.Tags {
		if err := tag.Validate(); err != nil {
			return fmt.Errorf("tag [%d] must be %w", i, err)
		}
	}
	return nil
}

// InternalizeOutput represents the output for the internalize action
type InternalizeOutput struct {
	OutputIndex         uint32              `json:"outputIndex"`
	Protocol            InternalizeProtocol `json:"protocol"`
	PaymentRemittance   *WalletPayment      `json:"paymentRemittance,omitempty"`
	InsertionRemittance *BasketInsertion    `json:"insertionRemittance,omitempty"`
}

// Validate checks if the InternalizeOutput is valid
func (output *InternalizeOutput) Validate() error {
	if output.Protocol == "" {
		return fmt.Errorf("protocol cannot be empty")
	}

	switch output.Protocol {
	case WalletPaymentProtocol:
		if output.PaymentRemittance == nil {
			return fmt.Errorf("payment remittance cannot be nil for wallet payment protocol")
		}
		if err := output.PaymentRemittance.Validate(); err != nil {
			return fmt.Errorf("wrong paymentRemittance: %w", err)
		}
	case BasketInsertionProtocol:
		if output.InsertionRemittance == nil {
			return fmt.Errorf("insertion remittance cannot be nil for basket insertion protocol")
		}
		if err := output.InsertionRemittance.Validate(); err != nil {
			return fmt.Errorf("wrong insertionRemittance: %w", err)
		}
	default:
		return fmt.Errorf("invalid protocol: %s", output.Protocol)
	}
	return nil
}

// InternalizeActionArgs represents the arguments for the internalize action
type InternalizeActionArgs struct {
	Tx             primitives.ExplicitByteArray   `json:"tx"`
	Outputs        []*InternalizeOutput           `json:"outputs"`
	Description    primitives.String5to2000Bytes  `json:"description"`
	Labels         []primitives.StringUnder300    `json:"labels"`
	SeekPermission *primitives.BooleanDefaultTrue `json:"seekPermission"`
}

// InternalizeActionResult represents the result of an internalize action with a status indicating if it was accepted or not.
type InternalizeActionResult struct {
	Accepted bool   `json:"accepted"`
	IsMerge  bool   `json:"isMerge"`
	TxID     string `json:"txid"`
	Satoshis int64  `json:"satoshis"`
}
