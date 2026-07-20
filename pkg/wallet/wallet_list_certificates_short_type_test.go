package wallet_test

import (
	"encoding/base64"
	"testing"

	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	certs_testabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// Regression for #769: short certificate types from the TS ecosystem must round-trip
// through acquire → storage → list without zero-pad re-encode corruption.
func (s *WalletTestSuite) Test_ListCertificates_ShortCertificateTypeRoundTrip() {
	t := s.T()

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	aliceWallet := given.AliceWalletWithStorage(s.StorageType)

	const shortTypeB64 = "Q29tbW9uU291cmNlIGlkZW50aXR5" // "CommonSource identity" (21 bytes)
	rawType, err := base64.StdEncoding.DecodeString(shortTypeB64)
	require.NoError(t, err)
	require.Len(t, rawType, 21)

	var shortType wallet.CertificateType
	copy(shortType[:], rawType)

	args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
	args.Type = shortType

	// when: acquire stores the cert (must use trimmed base64, not 32-byte pad)
	cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)
	require.NoError(t, err)
	require.NotNil(t, cert)
	require.Equal(t, shortType, cert.Type)

	// and: list by the short (zero-padded in [32]byte) type
	listResult, err := aliceWallet.ListCertificates(t.Context(), wallet.ListCertificatesArgs{
		Types: []wallet.CertificateType{shortType},
		Limit: to.Ptr(uint32(10)),
	}, fixtures.DefaultOriginator)

	// then:
	require.NoError(t, err)
	require.Equal(t, uint32(1), listResult.TotalCertificates)
	require.Len(t, listResult.Certificates, 1)
	require.Equal(t, shortType, listResult.Certificates[0].Type)
	// wire form after encode must still be the original short base64
	require.Equal(t, shortTypeB64, primitives.EncodeBytes32Base64([32]byte(listResult.Certificates[0].Type)))
}

func TestShortCertificateType_WireForm(t *testing.T) {
	t.Parallel()

	const shortTypeB64 = "Q29tbW9uU291cmNlIGlkZW50aXR5"
	rawType, err := base64.StdEncoding.DecodeString(shortTypeB64)
	require.NoError(t, err)

	var shortType wallet.CertificateType
	copy(shortType[:], rawType)

	// go-sdk CertificateType.Base64() still pads — toolbox helper must not.
	require.NotEqual(t, shortTypeB64, shortType.Base64())
	require.Equal(t, shortTypeB64, primitives.EncodeBytes32Base64([32]byte(shortType)))
}
