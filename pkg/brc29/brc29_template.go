package brc29

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
)

// LockForCounterparty generates a locking script for a BRC29 address derived from the sender, key ID, and recipient public key.
//
// Arguments:
//   - sender: the sender key. Can be a private key hex or wif or a key deriver or ec.PrivateKey.
//   - keyID: the key ID.
//   - recipient: the recipient key. This is the public key from private key that will be able to unlock it later. Can be a public key hex or a key deriver or ec.PublicKey.
//   - opts: additional options.
//
// Example:
// 1. LockForCounterparty with hexes
// ```go
// lockingScript, err := LockForCounterparty(PrivHex("ab..."), keyID, PubHex("cd..."))
// ```
//
// 2. LockForCounterparty with key derivers
// ```go
// var senderKeyDeriver *sdk.KeyDeriver = ...
// var recipientKeyDeriver *sdk.KeyDeriver = ...
//
// lockingScript, err := LockForCounterparty(senderKeyDeriver, keyID, recipientKeyDeriver)
// ```
//
// 3. LockForCounterparty with ec private and public keys
// ```go
// var priv ec.PrivateKey = ...
// var pub ec.PublicKey = ...
//
// lockingScript, err := LockForCounterparty(priv, keyID, pub)
// ```
func LockForCounterparty[SenderKey CounterpartyPrivateKey, RecipientKey CounterpartyPublicKey](senderPrivateKeySource SenderKey, keyID KeyID, recipientPublicKeySource RecipientKey, opts ...func(*lockOptions)) (*script.Script, error) {
	address, err := AddressForCounterparty(senderPrivateKeySource, keyID, recipientPublicKeySource, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate BRC29 address to lock the output: %w", err)
	}

	lockingScript, err := p2pkh.Lock(address)
	if err != nil {
		return nil, fmt.Errorf("failed to lock the output with BRC29: %w", err)
	}
	return lockingScript, nil
}

// LockForSelf generates a locking script for a BRC29 address derived from the sender's public key, key ID, and the recipient's (self) private key.
//
// This is the self-locking variant and uses AddressForSelf under the hood. If you need to
// lock for a counterparty using your private key and their public key, use LockForCounterparty instead.
//
// Arguments:
//   - sender: the sender key. Can be a public key hex or a key deriver or ec.PublicKey.
//   - keyID: the key ID.
//   - self: the recipient private key. This is the private key for which the output will be locked. Can be a private key hex or wif or a key deriver or ec.PrivateKey.
//   - opts: additional options.
//
// Example:
// 1. LockForSelf with hexes
// ```go
// lockingScript, err := LockForSelf(PubHex("ab..."), keyID, PrivHex("cd..."))
// ```
//
// 2. LockForSelf with key derivers
// ```go
// var senderKeyDeriver *sdk.KeyDeriver = ...
// var selfKeyDeriver *sdk.KeyDeriver = ...
//
// lockingScript, err := LockForSelf(senderKeyDeriver, keyID, selfKeyDeriver)
// ```
//
// 3. LockForSelf with ec private and public keys
// ```go
// var priv ec.PrivateKey = ...
// var pub ec.PublicKey = ...
//
// lockingScript, err := LockForSelf(pub, keyID, priv)
// ```
func LockForSelf[SenderKey CounterpartyPublicKey, SelfKey CounterpartyPrivateKey](senderPublicKeySource SenderKey, keyID KeyID, selfPrivateKeySource SelfKey, opts ...func(*lockOptions)) (*script.Script, error) {
	address, err := AddressForSelf(senderPublicKeySource, keyID, selfPrivateKeySource, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate BRC29 address to lock the output: %w", err)
	}

	lockingScript, err := p2pkh.Lock(address)
	if err != nil {
		return nil, fmt.Errorf("failed to lock the output with BRC29: %w", err)
	}
	return lockingScript, nil
}

var _ transaction.UnlockingScriptTemplate = (*UnlockingScriptTemplate)(nil)

// UnlockingScriptTemplate is transaction.UnlockingScriptTemplate implementation for BRC29.
type UnlockingScriptTemplate struct {
	unlocker *p2pkh.P2PKH
}

// Unlock generates an unlocking script for a BRC29 address derived from the sender, key ID, and recipient private key.
//
// Arguments:
//   - senderPublicKeySource: the sender key. Can be a public key hex or a key deriver or ec.PublicKey.
//   - keyID: the key ID.
//   - recipientPrivateKeySource: the recipient key. This is the private key for which the output was locked for. Can be a private key hex or wif or a key deriver or ec.PrivateKey.
//   - opts: additional options.
//
// Additional options:
//   - WithSigHash: the sighash type to use for signing.
//
// Example:
// 1. Unlock with hexes
// ```go
// unlockingScriptTemplate, err := Unlock(PubHex("ab..."), keyID, PrivHex("cd..."))
// ```
// 2. Unlock with key derivers
// ```go
// var senderKeyDeriver *sdk.KeyDeriver = ...
// var recipientKeyDeriver *sdk.KeyDeriver = ...
//
// unlockingScriptTemplate, err := Unlock(senderKeyDeriver, keyID, recipientKeyDeriver)
// ```
// 3. Unlock with ec private and public keys
// ```go
// var priv ec.PrivateKey = ...
// var pub ec.PublicKey = ...
//
// unlockingScriptTemplate, err := Unlock(pub, keyID, priv)
// ```
// 4. Unlock with sig hash
// ```go
// unlockingScriptTemplate, err := Unlock(PubHex("ab..."), keyID, PrivHex("cd..."), WithSigHash(SigHashAll))
// ```
func Unlock[SenderKey CounterpartyPublicKey, RecipientKey CounterpartyPrivateKey](senderPublicKeySource SenderKey, keyID KeyID, recipientPrivateKeySource RecipientKey, opts ...func(*unlockOptions)) (*UnlockingScriptTemplate, error) {
	options := &unlockOptions{}

	for _, opt := range opts {
		opt(options)
	}

	key, err := deriveRecipientPrivateKey(senderPublicKeySource, keyID, recipientPrivateKeySource)
	if err != nil {
		return nil, fmt.Errorf("failed to derive recipient private key to unlock the input: %w", err)
	}

	unlocker, err := p2pkh.Unlock(key, options.sigHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create BRC29 unlocker: %w", err)
	}

	return &UnlockingScriptTemplate{
		unlocker: unlocker,
	}, nil
}

// Sign signs the transaction input with BRC29.
func (u *UnlockingScriptTemplate) Sign(tx *transaction.Transaction, inputIndex uint32) (*script.Script, error) {
	unlockingScript, err := u.unlocker.Sign(tx, inputIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to sign input %d with BRC29: %w", inputIndex, err)
	}
	return unlockingScript, nil
}

// EstimateLength estimates the length of the BRC29 unlocking script for the input.
func (u *UnlockingScriptTemplate) EstimateLength(tx *transaction.Transaction, inputIndex uint32) uint32 {
	return u.unlocker.EstimateLength(tx, inputIndex)
}
