package wallet_test

import (
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	certs_testabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func (s *WalletTestSuite) Test_ProveCertificate() {
	t := s.T()

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
}
