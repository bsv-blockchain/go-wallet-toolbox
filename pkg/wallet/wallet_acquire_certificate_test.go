package wallet_test

import (
	"encoding/base64"
	"testing"

	"github.com/bsv-blockchain/go-sdk/auth/certificates"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/transaction"
	tu "github.com/bsv-blockchain/go-sdk/util/test_util"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func (ws *WalletTestSuite) TestAcquireCert() {
	t := ws.T()

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	aliceWallet := given.AliceWalletWithStorage(ws.StorageType)

	// and:
	args := CreateSampleAcquireCertificateArgs(t, aliceWallet)

	// when:
	cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

	// then:
	require.NoError(t, err)
	require.NotNil(t, cert)
}

func CreateSampleCertificate(t *testing.T, aliceWallet certificates.CertifierWallet) *certificates.Certificate {
	// Generate subject
	key, err := aliceWallet.GetPublicKey(t.Context(), wallet.GetPublicKeyArgs{IdentityKey: true}, fixtures.DefaultOriginator)
	require.NoError(t, err)
	require.NotEmpty(t, key)

	// Prepare type and serial number
	typeBytes := tu.GetByte32FromString("test-certificate-type")
	sampleType := wallet.StringBase64(base64.StdEncoding.EncodeToString(typeBytes[:]))

	serialBytes := tu.GetByte32FromString("test-serial-number")
	sampleSerialNumber := wallet.StringBase64(base64.StdEncoding.EncodeToString(serialBytes[:]))

	// Create a revocation outpoint
	txid := make([]byte, 32) // all zeros
	var outpoint transaction.Outpoint
	copy(outpoint.Txid[:], txid)
	outpoint.Index = 1
	sampleRevocationOutpoint := &outpoint

	// Prepare fields
	sampleFields := map[wallet.CertificateFieldNameUnder50Bytes]wallet.StringBase64{
		wallet.CertificateFieldNameUnder50Bytes("name"):         wallet.StringBase64("Alice"),
		wallet.CertificateFieldNameUnder50Bytes("email"):        wallet.StringBase64("alice@example.com"),
		wallet.CertificateFieldNameUnder50Bytes("organization"): wallet.StringBase64("Example Corp"),
	}

	// Build certificate
	certificate := &certificates.Certificate{
		Type:               sampleType,
		SerialNumber:       sampleSerialNumber,
		Subject:            to.Value(key.PublicKey),
		RevocationOutpoint: sampleRevocationOutpoint,
		Fields:             sampleFields,
		Signature:          nil, // will be set by Sign
	}

	// Sign the certificate using Alice's wallet
	err = certificate.Sign(t.Context(), aliceWallet)
	require.NoError(t, err)

	err = certificate.Verify(t.Context())
	require.NoError(t, err)

	return certificate
}

func CreateSampleAcquireCertificateArgs(t *testing.T, aliceWallet certificates.CertifierWallet) wallet.AcquireCertificateArgs {
	cert := CreateSampleCertificate(t, aliceWallet)

	sig, err := ec.ParseSignature(cert.Signature)
	require.NoError(t, err)
	require.NotNil(t, sig)

	fields := make(map[string]string)
	for k, v := range cert.Fields {
		fields[to.String(k)] = to.String(v)
	}

	// Build AcquireCertificateArgs
	args := wallet.AcquireCertificateArgs{
		AcquisitionProtocol: wallet.AcquisitionProtocolDirect,
		Type:                (wallet.CertificateType)([]byte(cert.Type)),
		SerialNumber:        to.Ptr(wallet.SerialNumber([]byte(cert.SerialNumber))),
		Certifier:           &cert.Certifier,
		RevocationOutpoint:  cert.RevocationOutpoint,
		Fields:              fields,
		Signature:           sig,
	}

	return args
}
