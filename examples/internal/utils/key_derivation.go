package utils

import (
	"encoding/base64"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

const (
	defaultBase64Prefix = "SfKxPIJNgdI="
	defaultBase64Suffix = "NaGLC6fMH50="
)

type PaymentRemittance struct {
	DerivationPrefix  []byte `json:"derivationPrefix"`
	DerivationSuffix  []byte `json:"derivationSuffix"`
	SenderIdentityKey string `json:"senderIdentityKey"`
}

// DerivationBytesResult represents the result of derivation bytes calculation
type DerivationBytesResult struct {
	DerivationPrefix []byte `json:"derivationPrefix"`
	DerivationSuffix []byte `json:"derivationSuffix"`
}

// DerivationParts creates derivation parts with default prefix and suffix
func DerivationParts() *sdk.Payment {
	prefix := "" // empty string will use default base64 prefix
	suffix := "" // empty string will use default base64 suffix
	bytes := derivationBytes(prefix, suffix)

	_, publicKey := sdk.AnyoneKey()

	paymentRemittance := &sdk.Payment{
		DerivationPrefix:  bytes.DerivationPrefix,
		DerivationSuffix:  bytes.DerivationSuffix,
		SenderIdentityKey: publicKey,
	}

	return paymentRemittance
}

func derivationBytes(prefix, suffix string) DerivationBytesResult {
	var derivationPrefix []byte
	var derivationSuffix []byte
	var err error

	if prefix == "" {
		prefix = defaultBase64Prefix
	}

	derivationPrefix, err = base64.StdEncoding.DecodeString(prefix)
	if err != nil {
		panic(fmt.Errorf("failed to decode default base64 prefix: %w", err))
	}

	if suffix == "" {
		suffix = defaultBase64Suffix
	}

	derivationSuffix, err = base64.StdEncoding.DecodeString(suffix)
	if err != nil {
		panic(fmt.Errorf("failed to decode default base64 suffix: %w", err))
	}

	return DerivationBytesResult{
		DerivationPrefix: derivationPrefix,
		DerivationSuffix: derivationSuffix,
	}
}
