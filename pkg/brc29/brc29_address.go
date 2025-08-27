package brc29

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/script"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// MyAddress generates a blockchain address according to BRC29 specification.
// It is meant to be used by the recipient to generate a BRC29 address for himself.
// If you are a sender, and you want to generate an address to send funds for a recipient, use brc29.YourAddress instead.
//
// The sender key can be a public key hex or a key deriver or ec.PublicKey.
// The recipient key can be a private key hex string or a key deriver or ec.PrivateKey.
//
// Additional options allow setting the address network to mainnet or testnet. By default, mainnet address is generated.
//
// Examples:
// 1. Use key hexes to generate an address
// ```go
// address, err := brc29.MyAddress(brc29.PubHex("ab..."), keyID, brc29.PrivHex("cd..."))
// ```
// 2. Use key derivers to generate an address
// ```go
// var senderDeriver *sdk.KeyDeriver = ...
// var recipientDeriver *sdk.KeyDeriver = ...
//
// address, err := brc29.MyAddress(senderDeriver, keyID, recipientDeriver)
// ```
// 3. Use ec.PublicKey and ec.PrivateKey to generate an address
// ```go
// var pub *ec.PublicKey = ...
// var priv *ec.PrivateKey = ...
//
// address, err := brc29.MyAddress(brc29.PubHex("ab..."), keyID, brc29.PrivHex("cd..."))
// ```
// 4. Use WIF string to generate an address
// ```go
// address, err := brc29.MyAddress(pub, keyID, brc29.WIF("ab..."))
// ```
// 5. Testnet address
// ```go
// address, err := brc29.MyAddress(brc29.PubHex("ab..."), keyID, brc29.PrivHex("cd..."), brc29.WithTestNet())
// ```
func MyAddress[S CounterpartyPublicKey, R CounterpartyPrivateKey](senderPublicKey S, keyID KeyID, myPrivateKey R, opts ...func(*lockOptions)) (*script.Address, error) {
	options := &lockOptions{
		mainNet: true,
	}

	for _, opt := range opts {
		opt(options)
	}

	key, err := deriveRecipientPrivateKey(senderPublicKey, keyID, myPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key for BRC29 address: %w", err)
	}

	address, err := script.NewAddressFromPublicKey(key.PubKey(), options.mainNet)
	if err != nil {
		return nil, fmt.Errorf("failed to create brc29 address from public key: %w", err)
	}
	return address, nil
}

// YourAddress generates a blockchain address according to BRC29 specification.
// It is meant to be used by the sender to generate a BRC29 address for a recipient.
// If you are a recipient, and you want to generate an address to pass it to a sender, use brc29.MyAddress instead.
//
// The sender key can be a private key hex string or a key deriver or ec.PrivateKey.
// The recipient key can be a public key hex or a key deriver or ec.PublicKey.
//
// Additional options allow setting the address network to mainnet or testnet.
//
// Examples:
// 1. Use key hexes to generate an address
// ```go
// address, err := brc29.YourAddress(brc29.PrivHex("ab..."), keyID, brc29.PubHex("cd..."))
// ```
// 2. Use key derivers to generate an address
// ```go
// var senderDeriver *sdk.KeyDeriver = ...
// var recipientDeriver *sdk.KeyDeriver = ...
//
// address, err := brc29.YourAddress(senderDeriver, keyID, recipientDeriver)
// ```
// 3. Use ec.PrivateKey and ec.PublicKey to generate an address
// ```go
// var priv *ec.PrivateKey = ...
// var pub *ec.PublicKey = ...
//
// address, err := brc29.YourAddress(brc29.PrivHex("ab..."), keyID, brc29.PubHex("cd..."))
// ```
// 4. Use WIF string to generate an address
// ```go
// address, err := brc29.YourAddress(brc29.WIF("ab..."), keyID, pub)
// ```
// 5. Testnet address
// ```go
// address, err := brc29.YourAddress(brc29.PrivHex("ab..."), keyID, brc29.PubHex("cd..."), brc29.WithTestNet())
// ```
func YourAddress[S CounterpartyPrivateKey, R CounterpartyPublicKey](senderPrivateKey S, keyID KeyID, yourPublicKey R, opts ...func(*lockOptions)) (*script.Address, error) {
	options := &lockOptions{
		mainNet: true,
	}

	for _, opt := range opts {
		opt(options)
	}

	if err := keyID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid key ID: %w", err)
	}

	senderKeyDeriver, err := toKeyDeriver(senderPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create sender key deriver from %T: %w", senderPrivateKey, err)
	}

	recipientIdentityKey, err := toIdentityKey(yourPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipient identity key from %T: %w", yourPublicKey, err)
	}

	key, err := senderKeyDeriver.DerivePublicKey(Protocol, keyID.String(), sdk.Counterparty{
		Type:         sdk.CounterpartyTypeOther,
		Counterparty: recipientIdentityKey,
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key for recipient for BRC29 address: %w", err)
	}

	address, err := script.NewAddressFromPublicKey(key, options.mainNet)
	if err != nil {
		return nil, fmt.Errorf("failed to create brc29 address for recipient from public key: %w", err)
	}
	return address, nil
}
