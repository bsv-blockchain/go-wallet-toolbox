package core

import (
	"encoding/base64"
	"fmt"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

const (
	defaultPrefix = "SfKxPIJNgdI="
	defaultSuffix = "NaGLC6fMH50="
)

// DerivationPartsResult represents the result of derivation parts calculation
type DerivationPartsResult struct {
	KeyID             string             `json:"keyId"`
	IdentityKey       string             `json:"identityKey"`
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
func DerivationParts() DerivationPartsResult {
	prefix := "somestring" // set this to any string you want to change the address
	suffix := "somestring" // set this to any string you want to change the address

	bytes := DerivationBytes(prefix, suffix, nil)

	derivationPrefix := base64.StdEncoding.EncodeToString(bytes.DerivationPrefix)
	derivationSuffix := base64.StdEncoding.EncodeToString(bytes.DerivationSuffix)

	// TODO: here we will get the identity key of the sender for now just hardcoding a value
	senderIdentityKey := "020c0ca23c75f7312bad0c5d81bff858bdcf468d3ad69a60b46ae90cafef557b03"

	paymentRemittance := &sdk.Payment{
		DerivationPrefix:  derivationPrefix,
		DerivationSuffix:  derivationSuffix,
		SenderIdentityKey: senderIdentityKey,
	}

	return DerivationPartsResult{
		KeyID:             KeyID(derivationPrefix, derivationSuffix),
		IdentityKey:       senderIdentityKey,
		PaymentRemittance: paymentRemittance,
	}
}

// KeyID creates a key ID from derivation prefix and suffix
func KeyID(derivationPrefix, derivationSuffix string) string {
	return fmt.Sprintf("%s %s", derivationPrefix, derivationSuffix)
}

// DerivationBytes creates derivation bytes from prefix and suffix
func DerivationBytes(prefix, suffix string, opts *DerivationBytesOpts) DerivationBytesResult {
	// Default to old prefix/suffix (base64 encoded)
	derivationPrefix, _ := base64.StdEncoding.DecodeString(defaultPrefix)
	derivationSuffix, _ := base64.StdEncoding.DecodeString(defaultSuffix)

	// Override with provided prefix/suffix if given
	if prefix != "" {
		encoding := "utf8"
		if opts != nil && opts.Encoding != "" {
			encoding = opts.Encoding
		}

		switch encoding {
		case "base64":
			derivationPrefix, _ = base64.StdEncoding.DecodeString(prefix)
		default:
			derivationPrefix = []byte(prefix)
		}
	}

	if suffix != "" {
		encoding := "utf8"
		if opts != nil && opts.Encoding != "" {
			encoding = opts.Encoding
		}

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
