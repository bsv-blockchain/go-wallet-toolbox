package brc29

import (
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/sdk"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
)

// Lock generates a locking script for a BRC29 address derived from the sender, key ID, and recipient public key.
//
// Arguments:
//   - sender: the sender key. Can be a private key hex string or a key deriver or ec.PrivateKey.
//   - keyID: the key ID.
//   - recipient: the recipient key. This is the public key from private key that will be able to unlock it later. Can be a public key hex or a key deriver or ec.PublicKey.
//   - opts: additional options.
func Lock[SenderKey CounterpartyPrivateKey, RecipientKey CounterpartyPublicKey](sender SenderKey, keyID KeyID, recipient RecipientKey, opts ...func(*lockOptions)) (*script.Script, error) {
	address, err := Address(sender, keyID, recipient, opts...)
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
//   - senderPublicKeySource: the sender key. Can be a private key hex string or a key deriver or ec.PrivateKey.
//   - keyID: the key ID.
//   - recipientPrivateKeySource: the recipient key. This is the private key for which the output was locked for. Can be a private key hex string or a key deriver or ec.PrivateKey.
//   - opts: additional options.
//
// Additional options:
//   - WithSigHash: the sighash type to use for signing.
func Unlock[SenderKey CounterpartyPublicKey, RecipientKey CounterpartyPrivateKey](senderPublicKeySource SenderKey, keyID KeyID, recipientPrivateKeySource RecipientKey, opts ...func(*unlockOptions)) (*UnlockingScriptTemplate, error) {
	options := &unlockOptions{}

	for _, opt := range opts {
		opt(options)
	}

	senderIdentityKey, err := toIdentityKey(senderPublicKeySource)
	if err != nil {
		return nil, fmt.Errorf("failed to create sender identity key: %w", err)
	}

	recipientKeyDeriver, err := toKeyDeriver(recipientPrivateKeySource)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipient key deriver: %w", err)
	}

	err = keyID.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid key ID: %w", err)
	}

	key, err := recipientKeyDeriver.DerivePrivateKey(Protocol, keyID.String(), sdk.Counterparty{
		Type:         sdk.CounterpartyTypeOther,
		Counterparty: senderIdentityKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to derive BRC29 private key for unlocking: %w", err)
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
