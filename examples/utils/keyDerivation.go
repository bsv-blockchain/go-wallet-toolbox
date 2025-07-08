package utils

import (
	"encoding/base64"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

const (
	DefaultBase64Prefix = "SfKxPIJNgdI="
	DefaultBase64Suffix = "NaGLC6fMH50="
)

// DerivationPartsResult represents the result of derivation parts calculation
type DerivationPartsResult struct {
	KeyID             string       `json:"keyId"`
	IdentityKey       string       `json:"identityKey"`
	PaymentRemittance *sdk.Payment `json:"paymentRemittance"`
}

// DerivationBytesResult represents the result of derivation bytes calculation
type DerivationBytesResult struct {
	DerivationPrefix []byte `json:"derivationPrefix"`
	DerivationSuffix []byte `json:"derivationSuffix"`
}

// DerivationBytesOpts represents options for derivation bytes
type DerivationBytesOpts struct {
	Encoding string `json:"encoding"` // "utf8" or "base64"
}

// DerivationParts creates derivation parts with default prefix and suffix
// senderIdentityKey is optional - if not provided, a new random key will be generated
func DerivationParts(senderIdentityKey ...string) DerivationPartsResult {
	prefix := "" // empty string will use default base64 prefix
	suffix := "" // empty string will use default base64 suffix
	bytes := derivationBytes(prefix, suffix, nil)

	derivationPrefix := base64.StdEncoding.EncodeToString(bytes.DerivationPrefix)
	derivationSuffix := base64.StdEncoding.EncodeToString(bytes.DerivationSuffix)

	var identityKey string
	if len(senderIdentityKey) > 0 && senderIdentityKey[0] != "" {
		identityKey = senderIdentityKey[0]
	} else {
		_, publicKey := sdk.AnyoneKey()
		identityKey = publicKey.ToDERHex()
	}

	paymentRemittance := &sdk.Payment{
		DerivationPrefix:  derivationPrefix,
		DerivationSuffix:  derivationSuffix,
		SenderIdentityKey: identityKey,
	}

	return DerivationPartsResult{
		KeyID:             keyID(derivationPrefix, derivationSuffix),
		IdentityKey:       identityKey,
		PaymentRemittance: paymentRemittance,
	}
}

func keyID(derivationPrefix, derivationSuffix string) string {
	return fmt.Sprintf("%s %s", derivationPrefix, derivationSuffix)
}

func derivationBytes(prefix, suffix string, opts *DerivationBytesOpts) DerivationBytesResult {
	var derivationPrefix []byte
	var derivationSuffix []byte

	encoding := "utf8"
	if opts != nil && opts.Encoding != "" {
		encoding = opts.Encoding
	}

	if prefix == "" {
		derivationPrefix, _ = base64.StdEncoding.DecodeString(DefaultBase64Prefix)
	} else {
		switch encoding {
		case "base64":
			derivationPrefix, _ = base64.StdEncoding.DecodeString(prefix)
		default:
			derivationPrefix = []byte(prefix)
		}
	}

	if suffix == "" {
		derivationSuffix, _ = base64.StdEncoding.DecodeString(DefaultBase64Suffix)
	} else {
		switch encoding {
		case "base64":
			derivationSuffix, _ = base64.StdEncoding.DecodeString(suffix)
		default:
			derivationSuffix = []byte(suffix)
		}
	}

	return DerivationBytesResult{
		DerivationPrefix: derivationPrefix,
		DerivationSuffix: derivationSuffix,
	}
}
