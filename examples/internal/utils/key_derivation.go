package utils

import (
	"encoding/base64"

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
// senderIdentityKey is optional - if not provided, a new random key will be generated
func DerivationParts() PaymentRemittance {
	prefix := "" // empty string will use default base64 prefix
	suffix := "" // empty string will use default base64 suffix
	bytes := derivationBytes(prefix, suffix)

	derivationPrefix := base64.StdEncoding.EncodeToString(bytes.DerivationPrefix)
	derivationSuffix := base64.StdEncoding.EncodeToString(bytes.DerivationSuffix)

	var identityKey string
	_, publicKey := sdk.AnyoneKey()
	identityKey = publicKey.ToDERHex()

	paymentRemittance := &PaymentRemittance{
		DerivationPrefix:  []byte(derivationPrefix),
		DerivationSuffix:  []byte(derivationSuffix),
		SenderIdentityKey: identityKey,
	}

	return *paymentRemittance
}

func derivationBytes(prefix string, suffix string) DerivationBytesResult {
	var derivationPrefix []byte
	var derivationSuffix []byte

	if prefix == "" {
		derivationPrefix, _ = base64.StdEncoding.DecodeString(defaultBase64Prefix)
	} else {
		derivationPrefix = []byte(prefix)
	}

	if suffix == "" {
		derivationSuffix, _ = base64.StdEncoding.DecodeString(defaultBase64Suffix)
	} else {
		derivationSuffix = []byte(suffix)
	}

	return DerivationBytesResult{
		DerivationPrefix: derivationPrefix,
		DerivationSuffix: derivationSuffix,
	}
}
