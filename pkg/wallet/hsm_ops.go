package wallet

import (
	"context"
	"crypto/hmac"
	"errors"
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	hash "github.com/bsv-blockchain/go-sdk/primitives/hash"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/keyderiver"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/actions"
)

// errHSMLinkageProof is returned for RevealCounterpartyKeyLinkage in HSM mode.
// go-sdk builds the Schnorr proof as z = r + e*a from the root scalar.
// CKM_EC_SCALAR_CHILD_DERIVE keeps that scalar inside a key object.
var errHSMLinkageProof = errors.New("hsm counterparty linkage proof requires the root scalar (z = r + e*a); CKM_EC_SCALAR_CHILD_DERIVE does not export it")

func (w *Wallet) newCreateAction() *actions.CreateAction {
	return &actions.CreateAction{
		Logger:                  w.logger,
		KeyDeriver:              w.keyDeriver,
		Signer:                  w.hsm,
		Storage:                 w.storage,
		WalletOpts:              w.flags,
		PendingSignActionsCache: w.pendingSignActionsCache,
	}
}

func hsmCounterparty(counterparty sdk.Counterparty, fallback sdk.CounterpartyType) sdk.Counterparty {
	if counterparty.Type == sdk.CounterpartyUninitialized {
		return sdk.Counterparty{Type: fallback}
	}
	return counterparty
}

func hsmGetPublicKey(ctx context.Context, d keyderiver.Deriver, args sdk.GetPublicKeyArgs) (*sdk.GetPublicKeyResult, error) {
	if args.IdentityKey {
		return &sdk.GetPublicKeyResult{PublicKey: d.IdentityKey()}, nil
	}
	if args.ProtocolID.Protocol == "" || args.KeyID == "" {
		return nil, fmt.Errorf("protocolID and keyID are required if identityKey is false")
	}
	forSelf := args.ForSelf != nil && *args.ForSelf
	pub, err := d.DerivePublicKey(ctx, args.ProtocolID, args.KeyID, hsmCounterparty(args.Counterparty, sdk.CounterpartyTypeSelf), forSelf)
	if err != nil {
		return nil, err
	}
	return &sdk.GetPublicKeyResult{PublicKey: pub}, nil
}

func hsmEncrypt(ctx context.Context, d keyderiver.Deriver, args sdk.EncryptArgs) (*sdk.EncryptResult, error) {
	ciphertext, err := d.Encrypt(ctx, args.ProtocolID, args.KeyID, hsmCounterparty(args.Counterparty, sdk.CounterpartyTypeSelf), args.Plaintext)
	if err != nil {
		return nil, err
	}
	return &sdk.EncryptResult{Ciphertext: ciphertext}, nil
}

func hsmDecrypt(ctx context.Context, d keyderiver.Deriver, args sdk.DecryptArgs) (*sdk.DecryptResult, error) {
	plaintext, err := d.Decrypt(ctx, args.ProtocolID, args.KeyID, hsmCounterparty(args.Counterparty, sdk.CounterpartyTypeSelf), args.Ciphertext)
	if err != nil {
		return nil, err
	}
	return &sdk.DecryptResult{Plaintext: plaintext}, nil
}

func hsmCreateHMAC(ctx context.Context, d keyderiver.Deriver, args sdk.CreateHMACArgs) (*sdk.CreateHMACResult, error) {
	mac, err := d.CreateHMAC(ctx, args.ProtocolID, args.KeyID, hsmCounterparty(args.Counterparty, sdk.CounterpartyTypeSelf), args.Data)
	if err != nil {
		return nil, err
	}
	return &sdk.CreateHMACResult{HMAC: mac}, nil
}

func hsmVerifyHMAC(ctx context.Context, d keyderiver.Deriver, args sdk.VerifyHMACArgs) (*sdk.VerifyHMACResult, error) {
	mac, err := d.CreateHMAC(ctx, args.ProtocolID, args.KeyID, hsmCounterparty(args.Counterparty, sdk.CounterpartyTypeSelf), args.Data)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(mac[:], args.HMAC[:]) {
		return nil, sdk.ErrInvalidHMAC
	}
	return &sdk.VerifyHMACResult{Valid: true}, nil
}

func hsmCreateSignature(ctx context.Context, d keyderiver.Deriver, args sdk.CreateSignatureArgs) (*sdk.CreateSignatureResult, error) {
	var digest []byte
	if len(args.HashToDirectlySign) > 0 {
		digest = args.HashToDirectlySign
	} else {
		digest = hash.Sha256(args.Data)
	}
	sig, err := d.Sign(ctx, args.ProtocolID, args.KeyID, hsmCounterparty(args.Counterparty, sdk.CounterpartyTypeAnyone), digest)
	if err != nil {
		return nil, err
	}
	return &sdk.CreateSignatureResult{Signature: sig}, nil
}

func hsmVerifySignature(ctx context.Context, d keyderiver.Deriver, args sdk.VerifySignatureArgs) (*sdk.VerifySignatureResult, error) {
	if len(args.Data) == 0 && len(args.HashToDirectlyVerify) == 0 {
		return nil, fmt.Errorf("args.data or args.hashToDirectlyVerify must be valid")
	}
	var digest []byte
	if len(args.HashToDirectlyVerify) > 0 {
		digest = args.HashToDirectlyVerify
	} else {
		digest = hash.Sha256(args.Data)
	}
	forSelf := args.ForSelf != nil && *args.ForSelf
	pub, err := d.DerivePublicKey(ctx, args.ProtocolID, args.KeyID, hsmCounterparty(args.Counterparty, sdk.CounterpartyTypeSelf), forSelf)
	if err != nil {
		return nil, err
	}
	if args.Signature == nil {
		return nil, fmt.Errorf("signature is nil")
	}
	if !args.Signature.Verify(digest, pub) {
		return nil, sdk.ErrInvalidSignature
	}
	return &sdk.VerifySignatureResult{Valid: true}, nil
}

func hsmRevealSpecificKeyLinkage(ctx context.Context, d keyderiver.Deriver, args sdk.RevealSpecificKeyLinkageArgs) (*sdk.RevealSpecificKeyLinkageResult, error) {
	if args.Verifier == nil {
		return nil, fmt.Errorf("verifier public key is required")
	}
	prover := d.IdentityKey()
	counterpartyPub, err := hsmCounterpartyPublicKey(args.Counterparty, prover)
	if err != nil {
		return nil, err
	}
	linkage, err := d.RevealSpecificSecret(ctx, args.Counterparty, args.ProtocolID, args.KeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to reveal specific secret: %w", err)
	}
	encryptProtocol := sdk.Protocol{
		SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty,
		Protocol:      fmt.Sprintf("specific linkage revelation %d %s", args.ProtocolID.SecurityLevel, args.ProtocolID.Protocol),
	}
	verifier := sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: args.Verifier}
	encryptedLinkage, err := d.Encrypt(ctx, encryptProtocol, args.KeyID, verifier, linkage)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt linkage: %w", err)
	}
	encryptedProof, err := d.Encrypt(ctx, encryptProtocol, args.KeyID, verifier, []byte{0})
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt proof: %w", err)
	}
	return &sdk.RevealSpecificKeyLinkageResult{
		EncryptedLinkage:      encryptedLinkage,
		EncryptedLinkageProof: encryptedProof,
		Prover:                prover,
		Verifier:              args.Verifier,
		Counterparty:          counterpartyPub,
		ProtocolID:            args.ProtocolID,
		KeyID:                 args.KeyID,
		ProofType:             0,
	}, nil
}

func hsmCounterpartyPublicKey(counterparty sdk.Counterparty, prover *ec.PublicKey) (*ec.PublicKey, error) {
	switch counterparty.Type {
	case sdk.CounterpartyTypeSelf:
		return prover, nil
	case sdk.CounterpartyTypeAnyone:
		_, anyone := sdk.AnyoneKey()
		return anyone, nil
	case sdk.CounterpartyTypeOther:
		if counterparty.Counterparty == nil {
			return nil, fmt.Errorf("counterparty public key is required")
		}
		return counterparty.Counterparty, nil
	case sdk.CounterpartyUninitialized:
		return nil, fmt.Errorf("invalid counterparty type: %v", counterparty.Type)
	default:
		return nil, fmt.Errorf("invalid counterparty type: %v", counterparty.Type)
	}
}
