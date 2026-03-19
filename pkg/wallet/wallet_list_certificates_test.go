package wallet_test

import (
	"encoding/base64"
	"testing"

	sdkprimitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	certs_testabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func (s *WalletTestSuite) Test_ListCertificates() {
	t := s.T()

	s.Run("should return certificate that matches given filter", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		// and:
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotNil(t, cert)

		tests := map[string]struct {
			name string
			args wallet.ListCertificatesArgs
		}{
			"filter includes certificate types list only": {
				args: wallet.ListCertificatesArgs{
					Types: []wallet.CertificateType{args.Type},
				},
			},
			"filter includes certifiers types list only": {
				args: wallet.ListCertificatesArgs{
					Certifiers: []*sdkprimitives.PublicKey{args.Certifier},
				},
			},
			"filter includes privileged flag set to true": {
				args: wallet.ListCertificatesArgs{
					Privileged: to.Ptr(true),
				},
			},
			"filter includes non privileged reason": {
				args: wallet.ListCertificatesArgs{
					PrivilegedReason: "reason",
				},
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				// when:
				actualResult, err := aliceWallet.ListCertificates(t.Context(), tc.args, fixtures.DefaultOriginator)

				// then:
				require.NoError(t, err)
				require.Equal(t, uint32(1), actualResult.TotalCertificates)
				require.Len(t, actualResult.Certificates, 1)

				// and:
				first := actualResult.Certificates[0]
				keyring := map[string]string{"name": base64.StdEncoding.EncodeToString([]byte("Alice Example"))}
				certs_testabilities.AssertCertificateResultEquality(t, first, cert, keyring)
			})
		}
	})

	s.Run("should return all certificates that match given filters", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and: acquire two certificates of the same type/certifier
		args1 := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		cert1, err := aliceWallet.AcquireCertificate(t.Context(), args1, fixtures.DefaultOriginator)
		require.NoError(t, err)

		args2 := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		args2.Type = args1.Type
		args2.Certifier = args1.Certifier

		cert2, err := aliceWallet.AcquireCertificate(t.Context(), args2, fixtures.DefaultOriginator)
		require.NoError(t, err)

		// and:
		listCertificatesArgs := wallet.ListCertificatesArgs{
			Types:      []wallet.CertificateType{args1.Type},
			Certifiers: []*sdkprimitives.PublicKey{args1.Certifier},
		}

		// when:
		actualResult, err := aliceWallet.ListCertificates(t.Context(), listCertificatesArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.Len(t, actualResult.Certificates, 2)
		require.Equal(t, uint32(2), actualResult.TotalCertificates)

		// optional sanity check for contents
		require.Equal(t, wallet.CertificateResult{Certificate: *cert1, Keyring: cert1.Fields, Verifier: []byte(cert1.Certifier.ToDERHex())}, actualResult.Certificates[0])
		require.Equal(t, wallet.CertificateResult{Certificate: *cert2, Keyring: cert2.Fields, Verifier: []byte(cert2.Certifier.ToDERHex())}, actualResult.Certificates[1])
	})

	s.Run("should return all certificates when no filters are provided", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// acquire multiple certificates
		expectedCerts := make([]*wallet.Certificate, 0, 3)
		for range 3 {
			cert, err := aliceWallet.AcquireCertificate(t.Context(), certs_testabilities.CreateSampleAcquireCertificateArgs(t), fixtures.DefaultOriginator)
			require.NoError(t, err)
			require.NotNil(t, cert)
			expectedCerts = append(expectedCerts, cert)
		}

		// when:
		actualResult, err := aliceWallet.ListCertificates(t.Context(), wallet.ListCertificatesArgs{}, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.Equal(t, uint32(3), actualResult.TotalCertificates)
		require.Len(t, actualResult.Certificates, 3)

		// and:
		for idx, actualCert := range actualResult.Certificates {
			expectedCert := expectedCerts[idx]
			keyring := map[string]string{"name": base64.StdEncoding.EncodeToString([]byte("Alice Example"))}
			certs_testabilities.AssertCertificateResultEquality(t, actualCert, expectedCert, keyring)
		}
	})
}
