package testabilities

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
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

	return wallet.AcquireCertificateArgs{
		Type:                certType,
		Certifier:           CreateTestCertifier(t),
		AcquisitionProtocol: wallet.AcquisitionProtocolDirect,
		Fields:              map[string]string{"name": "Alice Example"},
		SerialNumber:        &serialNum,
		RevocationOutpoint:  CreateTestOutpoint(t),
		Signature:           CreateTestSignature(t),
	}
}

func AssertCertificateResultEquality(t *testing.T, actualCert wallet.CertificateResult, expectedCert *wallet.Certificate, keyring map[string]string) {
	require.Equal(t, actualCert.Certifier, expectedCert.Certifier)
	require.Equal(t, actualCert.Fields, expectedCert.Fields)
	require.Equal(t, keyring, actualCert.Keyring)
	require.Equal(t, actualCert.Signature.Serialize(), expectedCert.Signature.Serialize())
	require.Equal(t, actualCert.RevocationOutpoint.String(), expectedCert.RevocationOutpoint.String())
}
