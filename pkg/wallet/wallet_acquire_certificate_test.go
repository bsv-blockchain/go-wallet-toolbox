package wallet_test

import (
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
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

		// then:
		actual, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		testabilities.AssertWalletCertificateEquality(t, actual, args, aliceWallet)
	})

	s.Run("should fail when certifier is missing", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.Certifier = nil // missing certifier

		// when:
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, cert)
	})

	s.Run("should fail when signature is missing", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.Signature = nil // invalid

		// when:
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, cert)
	})

	s.Run("should fail when revocation outpoint is invalid", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		args := testabilities.CreateSampleAcquireCertificateArgs(t)
		args.RevocationOutpoint = nil // invalid

		// when:
		cert, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, cert)
	})

	s.Run("should not create a duplicate when certificate already exists", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		args := testabilities.CreateSampleAcquireCertificateArgs(t)

		first, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)
		require.NoError(t, err)
		require.NotNil(t, first)

		// when:
		second, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.Error(t, err)
		require.Nil(t, second)
	})
}
