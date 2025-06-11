package brc29

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
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
//	 brc29.Lock(wif,...)
type WIF string

// PrivateKey returns the private key from the WIF string.
func (w WIF) PrivateKey() (*ec.PrivateKey, error) {
	return ec.PrivateKeyFromWif(string(w))
}

// CounterpartyPublicKey represents a source of counterparty identity (public) key.
// Can be used with different types of sources:
//   - string: a public key in DER HEX format
//   - *sdk.KeyDeriver: a key deriver that can be used to derive the public key
//   - *ec.PublicKey: a public key object
type CounterpartyPublicKey interface {
	string | *sdk.KeyDeriver | *ec.PublicKey
}

// CounterpartyPrivateKey represents a source of counterparty private key.
// Can be used with different types of sources:
//   - string: a private key in DER HEX format
//   - WIF: a WIF string
//   - *sdk.KeyDeriver: a key deriver that can be used to derive the private key
//   - *ec.PrivateKey: a private key object
type CounterpartyPrivateKey interface {
	string | WIF | *sdk.KeyDeriver | *ec.PrivateKey
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
