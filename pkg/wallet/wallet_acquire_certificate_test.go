package wallet_test

import (
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"
)

func (s *WalletTestSuite) Test_AcquireCertificate() {
	t := s.T()

	s.Run("should return and store certificate in the storage based on given arguments", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		// and:
		key, err := aliceWallet.GetPublicKey(t.Context(), wallet.GetPublicKeyArgs{IdentityKey: true}, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotNil(t, key)

		// and:
		args := testabilities.CreateSampleAcquireCertificateArgs(t)

		// and:
		expectedCertificate := wallet.Certificate{
			Type:               args.Type,
			SerialNumber:       to.Value(args.SerialNumber),
			Subject:            key.PublicKey,
			Certifier:          args.Certifier,
			RevocationOutpoint: args.RevocationOutpoint,
			Fields:             args.Fields,
			Signature:          args.Signature,
		}

		// then:
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		require.Equal(t, cert, to.Ptr(expectedCertificate))
	})
}
