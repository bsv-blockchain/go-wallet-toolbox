package wallet_test

import (
	sdkprimitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/stretchr/testify/require"
)

func (s *WalletTestSuite) Test_ListCertificates() {
	t := s.T()

	s.Run("should return certificate based on given list certificates args", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotNil(t, cert)

		// when:
		listCertificatesArgs := wallet.ListCertificatesArgs{
			Types:      []wallet.CertificateType{args.Type},
			Certifiers: []*sdkprimitives.PublicKey{args.Certifier},
		}

		res, err := aliceWallet.ListCertificates(t.Context(), listCertificatesArgs, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.NotNil(t, res)
	})
}
