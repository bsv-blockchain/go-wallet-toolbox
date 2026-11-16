package brc29

import (
	"context"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sighash "github.com/bsv-blockchain/go-sdk/transaction/sighash"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// PublicDeriver derives BRC-29 public keys without returning a private key.
type PublicDeriver interface {
	// IdentityKey returns the deriver's root public key.
	IdentityKey() *ec.PublicKey
	// DerivePublicKey returns the BRC-42 child public key.
	DerivePublicKey(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, forSelf bool) (*ec.PublicKey, error)
}

// DerivedSigner signs with a BRC-42 child key that stays inside the signer.
type DerivedSigner interface {
	PublicDeriver
	// Sign signs digest with the caller's child private key.
	Sign(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, digest []byte) (*ec.Signature, error)
}

// LockForSelfDerived builds the P2PKH locking script for the caller's own
// BRC-29 child of sender.
//
// The child public key is derived with forSelf set, which matches
// LockForSelf when the recipient key is a go-sdk KeyDeriver.
func LockForSelfDerived(ctx context.Context, sender *ec.PublicKey, keyID KeyID, self PublicDeriver, opts ...func(*lockOptions)) (*script.Script, error) {
	address, err := addressForSelfDerived(ctx, sender, keyID, self, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate BRC29 address to lock the output: %w", err)
	}
	lockingScript, err := p2pkh.Lock(address)
	if err != nil {
		return nil, fmt.Errorf("failed to lock the output with BRC29: %w", err)
	}
	return lockingScript, nil
}

// LockForCounterpartyDerived builds the P2PKH locking script for recipient's
// BRC-29 child of sender.
//
// The child public key is derived with forSelf clear, which matches
// LockForCounterparty when the sender key is a go-sdk KeyDeriver.
func LockForCounterpartyDerived(ctx context.Context, sender PublicDeriver, keyID KeyID, recipient *ec.PublicKey, opts ...func(*lockOptions)) (*script.Script, error) {
	address, err := addressForCounterpartyDerived(ctx, sender, keyID, recipient, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate BRC29 address to lock the output: %w", err)
	}
	lockingScript, err := p2pkh.Lock(address)
	if err != nil {
		return nil, fmt.Errorf("failed to lock the output with BRC29: %w", err)
	}
	return lockingScript, nil
}

// UnlockDerived returns an unlocking template that signs with signer.
// The child private key is not materialized in this process.
//
// ctx is captured because transaction.UnlockingScriptTemplate.Sign has no
// context parameter. Pass a context that is still active when the
// transaction is signed. For a sign-action that is completed on a later
// request, use a context that is not cancelled when the create request ends.
func UnlockDerived(ctx context.Context, sender *ec.PublicKey, keyID KeyID, signer DerivedSigner, opts ...func(*unlockOptions)) (*UnlockingScriptTemplate, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sender == nil {
		return nil, fmt.Errorf("sender public key is required")
	}
	if signer == nil {
		return nil, fmt.Errorf("signer is required")
	}
	if err := keyID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid key ID: %w", err)
	}
	options := &unlockOptions{}
	for _, opt := range opts {
		opt(options)
	}
	flag := sighash.AllForkID
	if options.sigHash != nil {
		flag = *options.sigHash
	}
	counterparty := sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: sender}
	invoiceKeyID := keyID.String()
	pub, err := signer.DerivePublicKey(ctx, Protocol, invoiceKeyID, counterparty, true)
	if err != nil {
		return nil, fmt.Errorf("failed to derive BRC29 public key: %w", err)
	}
	return &UnlockingScriptTemplate{
		derived: &derivedUnlocker{
			signDigest: func(digest []byte) (*ec.Signature, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return signer.Sign(ctx, Protocol, invoiceKeyID, counterparty, digest)
			},
			pub:  pub,
			flag: flag,
		},
	}, nil
}

func addressForSelfDerived(ctx context.Context, sender *ec.PublicKey, keyID KeyID, self PublicDeriver, opts ...func(*lockOptions)) (*script.Address, error) {
	options := &lockOptions{mainNet: true}
	for _, opt := range opts {
		opt(options)
	}
	if sender == nil {
		return nil, fmt.Errorf("sender public key is required")
	}
	if self == nil {
		return nil, fmt.Errorf("recipient deriver is required")
	}
	if err := keyID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid key ID: %w", err)
	}
	pub, err := self.DerivePublicKey(ctx, Protocol, keyID.String(), sdk.Counterparty{
		Type:         sdk.CounterpartyTypeOther,
		Counterparty: sender,
	}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to derive BRC29 public key: %w", err)
	}
	address, err := script.NewAddressFromPublicKey(pub, options.mainNet)
	if err != nil {
		return nil, fmt.Errorf("failed to create brc29 address from public key: %w", err)
	}
	return address, nil
}

func addressForCounterpartyDerived(ctx context.Context, sender PublicDeriver, keyID KeyID, recipient *ec.PublicKey, opts ...func(*lockOptions)) (*script.Address, error) {
	options := &lockOptions{mainNet: true}
	for _, opt := range opts {
		opt(options)
	}
	if sender == nil {
		return nil, fmt.Errorf("sender deriver is required")
	}
	if recipient == nil {
		return nil, fmt.Errorf("recipient public key is required")
	}
	if err := keyID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid key ID: %w", err)
	}
	pub, err := sender.DerivePublicKey(ctx, Protocol, keyID.String(), sdk.Counterparty{
		Type:         sdk.CounterpartyTypeOther,
		Counterparty: recipient,
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key for recipient for BRC29 address: %w", err)
	}
	address, err := script.NewAddressFromPublicKey(pub, options.mainNet)
	if err != nil {
		return nil, fmt.Errorf("failed to create brc29 address for recipient from public key: %w", err)
	}
	return address, nil
}

type derivedUnlocker struct {
	// signDigest closes over the caller's context. transaction.UnlockingScriptTemplate.Sign
	// has no context parameter, and the template can outlive the create request.
	signDigest func([]byte) (*ec.Signature, error)
	pub        *ec.PublicKey
	flag       sighash.Flag
}

func (u *derivedUnlocker) sign(tx *transaction.Transaction, inputIndex uint32) (*script.Script, error) {
	if int(inputIndex) >= len(tx.Inputs) || tx.Inputs[inputIndex].SourceTxOutput() == nil {
		return nil, transaction.ErrEmptyPreviousTx
	}
	digest, err := tx.CalcInputSignatureHash(inputIndex, u.flag)
	if err != nil {
		return nil, fmt.Errorf("sighash: %w", err)
	}
	sig, err := u.signDigest(digest)
	if err != nil {
		return nil, fmt.Errorf("failed to sign input %d with BRC29: %w", inputIndex, err)
	}
	signature := sig.Serialize()
	sigBuf := make([]byte, 0, len(signature)+1)
	sigBuf = append(sigBuf, signature...)
	sigBuf = append(sigBuf, uint8(u.flag))

	out := &script.Script{}
	if err = out.AppendPushData(sigBuf); err != nil {
		return nil, fmt.Errorf("failed to push BRC29 signature: %w", err)
	}
	if err = out.AppendPushData(u.pub.Compressed()); err != nil {
		return nil, fmt.Errorf("failed to push BRC29 public key: %w", err)
	}
	return out, nil
}
