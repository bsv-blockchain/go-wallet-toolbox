package testabilities

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"testing"

	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"
)

func CreateTestSignature(t *testing.T) *primitives.Signature {
	t.Helper()
	rBytes := make([]byte, 32)
	sBytes := make([]byte, 32)

	_, err := rand.Read(rBytes)
	require.NoError(t, err)

	_, err = rand.Read(sBytes)
	require.NoError(t, err)

	return &primitives.Signature{
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

func CreateTestCertifier(t *testing.T) *primitives.PublicKey {
	t.Helper()
	privKey, err := primitives.NewPrivateKey()
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

	copy(certType[:], []byte("testType"))
	copy(serialNum[:], []byte("serialXYZ"))

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
