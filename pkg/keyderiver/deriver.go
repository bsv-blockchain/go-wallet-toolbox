package keyderiver

import (
	"context"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// Deriver performs BRC-42 and BRC-43 operations for one root key.
//
// Implementations take derivation inputs and return public keys, signatures,
// ciphertext, or a linkage value. They do not return a child private key or
// the root private key.
//
// DerivePublicKey with forSelf reports the caller's child point (root + h*G).
// Otherwise it reports the counterparty point plus h*G.
// Sign signs digest with the caller's child key. digest is already a hash.
// CreateHMAC, Encrypt, and Decrypt use the BRC-43 symmetric key.
// RevealSpecificSecret is the BRC-42 scalar h.
// RevealCounterpartySecret is the ECDH shared point used by BRC-69.
type Deriver interface {
	// IdentityKey returns the root public key.
	IdentityKey() *ec.PublicKey

	// DerivePublicKey returns the BRC-42 child public key.
	DerivePublicKey(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, forSelf bool) (*ec.PublicKey, error)

	// DeriveSymmetricKey returns the BRC-43 symmetric key.
	DeriveSymmetricKey(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty) (*ec.SymmetricKey, error)

	// Sign signs digest with the BRC-42 child private key.
	Sign(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, digest []byte) (*ec.Signature, error)

	// CreateHMAC authenticates data under the BRC-43 symmetric key.
	CreateHMAC(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, data []byte) ([32]byte, error)

	// Encrypt encrypts plaintext under the BRC-43 symmetric key.
	Encrypt(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, plaintext []byte) ([]byte, error)

	// Decrypt decrypts ciphertext produced under the same derivation inputs.
	Decrypt(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, ciphertext []byte) ([]byte, error)

	// RevealSpecificSecret returns the BRC-42 scalar for this interaction.
	RevealSpecificSecret(ctx context.Context, counterparty sdk.Counterparty, protocol sdk.Protocol, keyID string) ([]byte, error)

	// RevealCounterpartySecret returns the ECDH shared point with counterparty.
	RevealCounterpartySecret(ctx context.Context, counterparty sdk.Counterparty) (*ec.PublicKey, error)

	// Close releases resources held by the deriver.
	Close() error
}
