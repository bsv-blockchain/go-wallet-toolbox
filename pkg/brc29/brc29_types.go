package brc29

import (
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// ProtocolID represents the unique identifier for the BRC-29 protocol.
const ProtocolID = "3241645161d8"

// Protocol is the protocol BRC29
var Protocol = sdk.Protocol{
	SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
	Protocol:      ProtocolID,
}

// WIF represents a string holding private key in WIF format.
// To pass a string as WIF simply wrap it with WIF type.
//
// Example:
//
//		wif := brc29.WIF("<KEY>")
//	 ...
//	 brc29.LockForCounterparty(wif,...)
type WIF string

// PrivateKey returns the private key from the WIF string.
func (w WIF) PrivateKey() (*ec.PrivateKey, error) {
	return ec.PrivateKeyFromWif(string(w)) //nolint:wrapcheck
}

// PrivHex represents a string holding private key in HEX format.
// To pass a string as PrivHex simply wrap it with PrivHex type, like brc29.PrivHex("ab...").
type PrivHex string

// PrivateKey returns the private key from the PrivHex string.
func (k PrivHex) PrivateKey() (*ec.PrivateKey, error) {
	return ec.PrivateKeyFromHex(string(k)) //nolint:wrapcheck
}

// CounterpartyPrivateKey represents a source of counterparty private key.
// Can be used with different types of sources:
//   - PrivHex: a private key in HEX format - to pass it, you simply wrap string with PrivHex type, like PrivHex("ab...")
//   - WIF: a private key in WIF format string - to pass it, you simply wrap string with WIF type, like WIF("L1...")
//   - *ec.PrivateKey: a private key object
//   - *sdk.KeyDeriver: a key deriver that can be used to derive the private key
type CounterpartyPrivateKey interface {
	PrivHex | WIF | *ec.PrivateKey | *sdk.KeyDeriver
}

// PubHex represents a string holding a public key in DER HEX format.
// To pass a string as PubHex simply wrap it with PubHex type, like PubHex("ab...").
type PubHex string

// PublicKey returns the public key from the PubHex string.
func (k PubHex) PublicKey() (*ec.PublicKey, error) {
	return ec.PublicKeyFromString(string(k)) //nolint:wrapcheck
}

// CounterpartyPublicKey represents a source of counterparty identity (public) key.
// Can be used with different types of sources:
//   - PubHex: a public key in HEX format - to pass it, you simply wrap string with PubHex type, like PubHex("ab...")
//   - *sdk.KeyDeriver: a key deriver that can be used to derive the public key
//   - *ec.PublicKey: a public key object
type CounterpartyPublicKey interface {
	PubHex | *sdk.KeyDeriver | *ec.PublicKey
}

// KeyID represents a key ID for BRC29.
//
// Key ID is a combination of derivation prefix and derivation suffix.
type KeyID struct {
	DerivationPrefix string
	DerivationSuffix string
}

// Validate validates the key ID.
//
// The key ID must have a derivation prefix and derivation suffix.
func (k *KeyID) Validate() error {
	if k.DerivationPrefix == "" {
		return fmt.Errorf("invalid key id: derivation prefix is required")
	}
	if k.DerivationSuffix == "" {
		return fmt.Errorf("invalid key id: derivation suffix is required")
	}
	return nil
}

// String returns the string that can be used for derivation.
func (k *KeyID) String() string {
	return k.DerivationPrefix + " " + k.DerivationSuffix
}
