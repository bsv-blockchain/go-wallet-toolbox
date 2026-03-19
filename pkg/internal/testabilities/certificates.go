package testabilities

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
)

func CreateTestSignature(t *testing.T) *ec.Signature {
	t.Helper()
	rBytes := make([]byte, 32)
	sBytes := make([]byte, 32)

	_, err := rand.Read(rBytes)
	require.NoError(t, err)

	_, err = rand.Read(sBytes)
	require.NoError(t, err)

	return &ec.Signature{
		R: new(big.Int).SetBytes(rBytes),
		S: new(big.Int).SetBytes(sBytes),
	}
}

func CreateTestOutpoint(t *testing.T) *transaction.Outpoint {
	t.Helper()
	txid := make([]byte, 32)
	_, err := rand.Read(txid)
	require.NoError(t, err)

	outpoint, err := transaction.OutpointFromString(hex.EncodeToString(txid) + ".0")
	require.NoError(t, err)
	require.NotNil(t, outpoint)

	return outpoint
}

func CreateTestCertifier(t *testing.T) *ec.PublicKey {
	t.Helper()
	privKey, err := ec.NewPrivateKey()
	require.NoError(t, err)
	require.NotNil(t, privKey)

	certifier := privKey.PubKey()
	require.NotNil(t, certifier)
	return certifier
}

func CreateTestCertificateType(t *testing.T) wallet.CertificateType {
	t.Helper()
	var cert wallet.CertificateType
	certBytes := make([]byte, 32)

	_, err := rand.Read(certBytes)
	require.NoError(t, err)
	copy(cert[:], certBytes)

	return cert
}

func CreateTestCertificateSerialNumber(t *testing.T) wallet.SerialNumber {
	t.Helper()
	var serial wallet.SerialNumber
	serialBytes := make([]byte, 32)

	_, err := rand.Read(serialBytes)
	require.NoError(t, err)
	copy(serial[:], serialBytes)

	return serial
}

func CreateSamplePubKey(t *testing.T) *ec.PublicKey {
	t.Helper()
	priv, err := ec.NewPrivateKey()
	require.NoError(t, err)
	require.NotNil(t, priv)

	pub := priv.PubKey()
	require.NotNil(t, pub)
	return pub
}

func CreateSampleAcquireCertificateArgs(t *testing.T) wallet.AcquireCertificateArgs {
	t.Helper()

	nameValue := "name"
	nameValueB64 := base64.StdEncoding.EncodeToString([]byte("Alice Example"))

	return wallet.AcquireCertificateArgs{
		Type:                CreateTestCertificateType(t),
		Certifier:           CreateTestCertifier(t),
		AcquisitionProtocol: wallet.AcquisitionProtocolDirect,
		Fields:              map[string]string{nameValue: nameValueB64},
		SerialNumber:        to.Ptr(CreateTestCertificateSerialNumber(t)),
		RevocationOutpoint:  CreateTestOutpoint(t),
		Signature:           CreateTestSignature(t),
		KeyringRevealer:     &wallet.KeyringRevealer{Certifier: true},
		KeyringForSubject:   map[string]string{nameValue: nameValueB64},
	}
}

func AssertCertificateResultEquality(t *testing.T, actual wallet.CertificateResult, expected *wallet.Certificate, keyring map[string]string) {
	t.Helper()
	require.Equal(t, expected.Certifier, actual.Certifier)

	// Compare Fields
	require.Len(t, actual.Fields, len(expected.Fields), "Fields map length mismatch")
	require.Equal(t, expected.Fields, actual.Fields)

	// Compare Keyring
	require.Len(t, actual.Keyring, len(keyring), "Keyring map length mismatch")

	require.Equal(t, keyring, actual.Keyring)
	require.Equal(t, actual.Signature.Serialize(), expected.Signature.Serialize())
	require.Equal(t, actual.RevocationOutpoint.String(), expected.RevocationOutpoint.String())
}

type PublicKeyProvider interface {
	GetPublicKey(ctx context.Context, args wallet.GetPublicKeyArgs, _originator string) (*wallet.GetPublicKeyResult, error)
}

func AssertWalletCertificateEquality(t *testing.T, actual *wallet.Certificate, args wallet.AcquireCertificateArgs, aliceWallet PublicKeyProvider) {
	t.Helper()

	require.NotNil(t, actual)

	key, err := aliceWallet.GetPublicKey(t.Context(), wallet.GetPublicKeyArgs{IdentityKey: true}, fixtures.DefaultOriginator)
	require.NoError(t, err)
	require.NotNil(t, key)

	expected := &wallet.Certificate{
		Type:               args.Type,
		SerialNumber:       to.Value(args.SerialNumber),
		Subject:            key.PublicKey,
		Certifier:          args.Certifier,
		RevocationOutpoint: args.RevocationOutpoint,
		Fields:             args.Fields,
		Signature:          args.Signature,
	}

	require.Equal(t, expected, actual)
}
