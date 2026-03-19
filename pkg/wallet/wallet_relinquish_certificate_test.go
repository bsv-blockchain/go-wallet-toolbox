package wallet_test

import (
	"testing"

	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	certs_testabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func (s *WalletTestSuite) Test_RelinquishCertificate() {
	t := s.T()

	t.Run("should relinquish certificate that matches given filter", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args1 := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args1, fixtures.DefaultOriginator)
		require.NoError(t, err)

		// and:
		filter := sdk.RelinquishCertificateArgs{
			Type:         cert.Type,
			SerialNumber: cert.SerialNumber,
			Certifier:    cert.Certifier,
		}

		// when:
		actualRelinquishCertificateResult, err := aliceWallet.RelinquishCertificate(t.Context(), filter, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, actualRelinquishCertificateResult)
		require.True(t, actualRelinquishCertificateResult.Relinquished)

		// and:
		actualListCertificatesResult, err := aliceWallet.ListCertificates(t.Context(), sdk.ListCertificatesArgs{
			Certifiers: []*primitives.PublicKey{cert.Certifier},
		}, fixtures.DefaultOriginator)

		require.NoError(t, err)
		require.NotNil(t, actualListCertificatesResult)

		require.Zero(t, actualListCertificatesResult.TotalCertificates)
		require.Empty(t, actualListCertificatesResult.Certificates)
	})

	t.Run("should return an error when a only one filter is provided", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		tests := map[string]struct {
			filter sdk.RelinquishCertificateArgs
		}{
			"filter includes types argument only": {
				filter: sdk.RelinquishCertificateArgs{
					Type: certs_testabilities.CreateTestCertificateType(t),
				},
			},
			"filter includes serial number argument only": {
				filter: sdk.RelinquishCertificateArgs{
					SerialNumber: certs_testabilities.CreateTestCertificateSerialNumber(t),
				},
			},
			"filter includes certifier argument only": {
				filter: sdk.RelinquishCertificateArgs{
					Certifier: certs_testabilities.CreateTestCertifier(t),
				},
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				// given:
				aliceWallet := given.AliceWalletWithStorage(s.StorageType)

				// and:
				args1 := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
				cert, err := aliceWallet.AcquireCertificate(t.Context(), args1, fixtures.DefaultOriginator)
				require.NoError(t, err)
				require.NotNil(t, cert)

				// and:

				// when:
				actualRelinquishCertificateResult, err := aliceWallet.RelinquishCertificate(t.Context(), tc.filter, fixtures.DefaultOriginator)

				// 	then:
				require.Error(t, err)
				require.Nil(t, actualRelinquishCertificateResult)

				// 	// and:
				actualListCertificatesResult, err := aliceWallet.ListCertificates(t.Context(), sdk.ListCertificatesArgs{
					Certifiers: []*primitives.PublicKey{cert.Certifier},
				}, fixtures.DefaultOriginator)

				require.NoError(t, err)
				require.NotNil(t, actualListCertificatesResult)

				require.NotZero(t, actualListCertificatesResult.TotalCertificates)
				require.NotEmpty(t, actualListCertificatesResult.Certificates)
			})
		}
	})

	t.Run("should return an error when no filters are provided", func(t *testing.T) {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args1 := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args1, fixtures.DefaultOriginator)
		require.NoError(t, err)

		// when:
		actualRelinquishCertificateResult, err := aliceWallet.RelinquishCertificate(t.Context(), sdk.RelinquishCertificateArgs{}, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, actualRelinquishCertificateResult)

		// and:
		actualListCertificatesResult, err := aliceWallet.ListCertificates(t.Context(), sdk.ListCertificatesArgs{
			Certifiers: []*primitives.PublicKey{cert.Certifier},
		}, fixtures.DefaultOriginator)

		require.NoError(t, err)
		require.NotNil(t, actualListCertificatesResult)

		require.NotZero(t, actualListCertificatesResult.TotalCertificates)
		require.NotEmpty(t, actualListCertificatesResult.Certificates)
	})
}
