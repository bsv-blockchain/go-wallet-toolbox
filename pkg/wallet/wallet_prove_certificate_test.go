package wallet_test

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	certs_testabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
)

func (s *WalletTestSuite) Test_ProveCertificate() {
	t := s.T()

	s.Run("should return ProveCertificateResult that match given filters - all fields set", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)

		// and:
		proveCertificatesArgs := wallet.ProveCertificateArgs{
			Certificate:      to.Value(cert),
			Verifier:         certs_testabilities.CreateSamplePubKey(t),
			FieldsToReveal:   []string{"name"},
			Privileged:       to.Ptr(true),
			PrivilegedReason: "reason",
		}

		// when:
		actualResult, err := aliceWallet.ProveCertificate(t.Context(), proveCertificatesArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, actualResult)
		require.Len(t, actualResult.KeyringForVerifier, 1)
	})

	s.Run("should return ProveCertificateResult that match given filters - certificate and verifier fields set", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)

		// and:
		proveCertificatesArgs := wallet.ProveCertificateArgs{
			Certificate: to.Value(cert),
			Verifier:    certs_testabilities.CreateSamplePubKey(t),
		}

		// when:
		actualResult, err := aliceWallet.ProveCertificate(t.Context(), proveCertificatesArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, actualResult)
		require.Len(t, actualResult.KeyringForVerifier, 1)
	})

	s.Run("should return an error when given filters missing necessary certificate, verifier fields", func() {
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
			args wallet.ProveCertificateArgs
		}{
			"filter includes fields to reveal list only": {
				args: wallet.ProveCertificateArgs{
					FieldsToReveal: []string{"name"},
				},
			},
			"filter includes privileged flag set only": {
				args: wallet.ProveCertificateArgs{
					Privileged: to.Ptr(true),
				},
			},
			"filter includes privileged reason set only": {
				args: wallet.ProveCertificateArgs{
					PrivilegedReason: "abcd",
				},
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				// when:
				actualResult, err := aliceWallet.ProveCertificate(t.Context(), tc.args, fixtures.DefaultOriginator)

				// then:
				require.Error(t, err)
				require.Nil(t, actualResult)
			})
		}
	})
}
