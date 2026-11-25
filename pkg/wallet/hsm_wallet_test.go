package wallet_test

import (
	"context"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/mocks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
)

// sdkDeriver is the software stand-in for keyderiver.Deriver. It delegates to
// go-sdk so the wallet dispatch can be compared with ProtoWallet. The HSM
// algorithm itself is tested in the hsm module.
type sdkDeriver struct {
	kd    *sdk.KeyDeriver
	proto *sdk.ProtoWallet
}

func newSDKDeriver(t *testing.T, key *ec.PrivateKey) *sdkDeriver {
	t.Helper()
	kd := sdk.NewKeyDeriver(key)
	proto, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypeKeyDeriver, KeyDeriver: kd})
	require.NoError(t, err)
	return &sdkDeriver{kd: kd, proto: proto}
}

func (s *sdkDeriver) IdentityKey() *ec.PublicKey { return s.kd.IdentityKey() }

func (s *sdkDeriver) DerivePublicKey(_ context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, forSelf bool) (*ec.PublicKey, error) {
	return s.kd.DerivePublicKey(protocol, keyID, counterparty, forSelf)
}

func (s *sdkDeriver) DeriveSymmetricKey(_ context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty) (*ec.SymmetricKey, error) {
	return s.kd.DeriveSymmetricKey(protocol, keyID, counterparty)
}

func (s *sdkDeriver) Sign(_ context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, digest []byte) (*ec.Signature, error) {
	priv, err := s.kd.DerivePrivateKey(protocol, keyID, counterparty)
	if err != nil {
		return nil, err
	}
	return priv.Sign(digest)
}

func (s *sdkDeriver) CreateHMAC(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, data []byte) ([32]byte, error) {
	res, err := s.proto.CreateHMAC(ctx, sdk.CreateHMACArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: counterparty},
		Data:           data,
	}, "")
	if err != nil {
		return [32]byte{}, err
	}
	return res.HMAC, nil
}

func (s *sdkDeriver) Encrypt(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, plaintext []byte) ([]byte, error) {
	res, err := s.proto.Encrypt(ctx, sdk.EncryptArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: counterparty},
		Plaintext:      plaintext,
	}, "")
	if err != nil {
		return nil, err
	}
	return res.Ciphertext, nil
}

func (s *sdkDeriver) Decrypt(ctx context.Context, protocol sdk.Protocol, keyID string, counterparty sdk.Counterparty, ciphertext []byte) ([]byte, error) {
	res, err := s.proto.Decrypt(ctx, sdk.DecryptArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: counterparty},
		Ciphertext:     ciphertext,
	}, "")
	if err != nil {
		return nil, err
	}
	return res.Plaintext, nil
}

func (s *sdkDeriver) RevealSpecificSecret(_ context.Context, counterparty sdk.Counterparty, protocol sdk.Protocol, keyID string) ([]byte, error) {
	return s.kd.RevealSpecificSecret(counterparty, protocol, keyID)
}

func (s *sdkDeriver) RevealCounterpartySecret(_ context.Context, counterparty sdk.Counterparty) (*ec.PublicKey, error) {
	return s.kd.RevealCounterpartySecret(counterparty)
}

func (s *sdkDeriver) Close() error { return nil }

func TestNewWithHSMMatchesSoftwareWallet(t *testing.T) {
	key := testusers.Alice.PrivateKey(t)
	storage := mocks.NewMockWalletStorageProvider(nil)
	software, err := wallet.New(defs.NetworkMainnet, key, storage)
	require.NoError(t, err)
	hsmWallet, err := wallet.NewWithHSM(defs.NetworkMainnet, newSDKDeriver(t, key), storage)
	require.NoError(t, err)

	originator := "test.example.com"
	protocol := sdk.Protocol{SecurityLevel: sdk.SecurityLevelEveryAppAndCounterparty, Protocol: "3241645161d8"}
	keyID := "prefix suffix"
	other := testusers.Bob.PublicKey(t)
	cp := sdk.Counterparty{Type: sdk.CounterpartyTypeOther, Counterparty: other}

	softPub, err := software.GetPublicKey(t.Context(), sdk.GetPublicKeyArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: cp},
		ForSelf:        new(true),
	}, originator)
	require.NoError(t, err)
	hsmPub, err := hsmWallet.GetPublicKey(t.Context(), sdk.GetPublicKeyArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: cp},
		ForSelf:        new(true),
	}, originator)
	require.NoError(t, err)
	require.True(t, softPub.PublicKey.IsEqual(hsmPub.PublicKey))

	identity, err := hsmWallet.GetPublicKey(t.Context(), sdk.GetPublicKeyArgs{IdentityKey: true}, originator)
	require.NoError(t, err)
	require.Equal(t, key.PubKey().ToDERHex(), identity.PublicKey.ToDERHex())

	payload := []byte("hsm wallet hmac")
	softMAC, err := software.CreateHMAC(t.Context(), sdk.CreateHMACArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: cp},
		Data:           payload,
	}, originator)
	require.NoError(t, err)
	hsmMAC, err := hsmWallet.CreateHMAC(t.Context(), sdk.CreateHMACArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: cp},
		Data:           payload,
	}, originator)
	require.NoError(t, err)
	require.Equal(t, softMAC.HMAC, hsmMAC.HMAC)

	digest := sdk.BytesList(payload)
	softSig, err := software.CreateSignature(t.Context(), sdk.CreateSignatureArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: cp},
		Data:           digest,
	}, originator)
	require.NoError(t, err)
	hsmSig, err := hsmWallet.CreateSignature(t.Context(), sdk.CreateSignatureArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: cp},
		Data:           digest,
	}, originator)
	require.NoError(t, err)
	require.Equal(t, softSig.Signature.Serialize(), hsmSig.Signature.Serialize())

	verified, err := hsmWallet.VerifySignature(t.Context(), sdk.VerifySignatureArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: cp},
		Data:           payload,
		Signature:      hsmSig.Signature,
		ForSelf:        new(true),
	}, originator)
	require.NoError(t, err)
	require.True(t, verified.Valid)

	plaintext := []byte("round trip")
	encrypted, err := hsmWallet.Encrypt(t.Context(), sdk.EncryptArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: cp},
		Plaintext:      plaintext,
	}, originator)
	require.NoError(t, err)
	decrypted, err := software.Decrypt(t.Context(), sdk.DecryptArgs{
		EncryptionArgs: sdk.EncryptionArgs{ProtocolID: protocol, KeyID: keyID, Counterparty: cp},
		Ciphertext:     encrypted.Ciphertext,
	}, originator)
	require.NoError(t, err)
	require.Equal(t, plaintext, []byte(decrypted.Plaintext))

	_, err = hsmWallet.RevealCounterpartyKeyLinkage(t.Context(), sdk.RevealCounterpartyKeyLinkageArgs{
		Counterparty: other,
		Verifier:     other,
	}, originator)
	require.Error(t, err)
}

func TestNewWithHSMRejectsNil(t *testing.T) {
	storage := mocks.NewMockWalletStorageProvider(nil)
	_, err := wallet.NewWithHSM(defs.NetworkMainnet, nil, storage)
	require.Error(t, err)
}
