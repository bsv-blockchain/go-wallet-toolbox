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
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
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

func CreateSampleAcquireCertificateArgs(t *testing.T) wallet.AcquireCertificateArgs {
	t.Helper()
	var (
		certType  wallet.CertificateType
		serialNum wallet.SerialNumber
	)

	certBytes := make([]byte, 32)
	sigBytes := make([]byte, 32)

	_, err := rand.Read(certBytes)
	require.NoError(t, err)

	_, err = rand.Read(sigBytes)
	require.NoError(t, err)

	copy(certType[:], certBytes)
	copy(serialNum[:], sigBytes)

	nameValue := "name"
	nameValueB64 := base64.StdEncoding.EncodeToString([]byte("Alice Example"))

	return wallet.AcquireCertificateArgs{
		Type:                certType,
		Certifier:           CreateTestCertifier(t),
		AcquisitionProtocol: wallet.AcquisitionProtocolDirect,
		Fields:              map[string]string{nameValue: nameValueB64},
		SerialNumber:        &serialNum,
		RevocationOutpoint:  CreateTestOutpoint(t),
		Signature:           CreateTestSignature(t),
		KeyringRevealer:     &wallet.KeyringRevealer{Certifier: true},
		KeyringForSubject:   map[string]string{nameValue: nameValueB64},
	}
}

func AssertCertificateResultEquality(t *testing.T, actual wallet.CertificateResult, expected *wallet.Certificate, keyring map[string]string) {
	t.Helper()
	require.Equal(t, actual.Certifier, expected.Certifier)

	// Compare Fields
	require.Equal(t, len(expected.Fields), len(actual.Fields), "Fields map length mismatch")
	require.Equal(t, expected.Fields, actual.Fields)

	// Compare Keyring
	require.Equal(t, len(keyring), len(actual.Keyring), "Keyring map length mismatch")

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

	require.Equal(t, actual, expected)
}
