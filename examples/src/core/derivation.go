package core

import (
	"encoding/base64"
	"fmt"

	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
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
	prefix := "someprefix" // set this to any string you want to change the address
	suffix := "somesuffix" // set this to any string you want to change the address
	bytes := DerivationBytes(prefix, suffix, nil)

	derivationPrefix := base64.StdEncoding.EncodeToString(bytes.DerivationPrefix)
	derivationSuffix := base64.StdEncoding.EncodeToString(bytes.DerivationSuffix)

	var identityKey string
	if len(senderIdentityKey) > 0 && senderIdentityKey[0] != "" {
		identityKey = senderIdentityKey[0]
	} else {
		privateKey, err := primitives.NewPrivateKey()
		if err != nil {
			panic(err)
		}
		identityKey = privateKey.PubKey().ToDERHex()
	}

	paymentRemittance := &sdk.Payment{
		DerivationPrefix:  derivationPrefix,
		DerivationSuffix:  derivationSuffix,
		SenderIdentityKey: identityKey,
	}

	return DerivationPartsResult{
		KeyID:             KeyID(derivationPrefix, derivationSuffix),
		IdentityKey:       identityKey,
		PaymentRemittance: paymentRemittance,
	}
}

// KeyID creates a key ID from derivation prefix and suffix
func KeyID(derivationPrefix, derivationSuffix string) string {
	return fmt.Sprintf("%s %s", derivationPrefix, derivationSuffix)
}

// DerivationBytes creates derivation bytes from prefix and suffix
func DerivationBytes(prefix, suffix string, opts *DerivationBytesOpts) DerivationBytesResult {
	var derivationPrefix []byte
	var derivationSuffix []byte

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
