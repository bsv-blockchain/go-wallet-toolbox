package wallet_test

import (
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	certs_testabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
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
		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)

		// then:
		actual, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

		// then:
		require.NoError(t, err)
		certs_testabilities.AssertWalletCertificateEquality(t, actual, args, aliceWallet)
	})

	s.Run("should fail when certifier is missing", func() {
		// given:
		given, cleanup := testabilities.Given(t)
		defer cleanup()

		// and:
		aliceWallet := given.AliceWalletWithStorage(s.StorageType)

		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
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

		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
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

		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
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

		args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)

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

func (s *WalletTestSuite) Test_AcquireCertificate_IssuanceProtocol() {
	t := s.T()

	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	args := certs_testabilities.CreateSampleAcquireCertificateArgs(t)
	args.AcquisitionProtocol = wallet.AcquisitionProtocolIssuance
	// and:
	// pubKeyHex := "02bbc996771abe50be940a9cfd91d6f28a70d139f340bedc8cdd4f236e5e9c9889"
	// pubKey, _ := ec.PublicKeyFromString(pubKeyHex)
	// requestID := make([]byte, 32)

	// and: create a certifier server wallet (the server that will issue certificates)
	certifierWallet := given.BobWalletWithStorage(s.StorageType) // Bob acts as certifier

	// and: create a test server with auth middleware
	certifierServer := given.
		CertifierServer().
		WithCertifierWallet(certifierWallet).
		Started()

	// and: create Alice's wallet (the client requesting a certificate)
	aliceWallet := given.AliceWalletWithStorage(s.StorageType)

	// and: prepare acquisition arguments with the test server URL
	// Get the certifier's identity key from Bob's wallet
	certifierKey, err := certifierWallet.GetPublicKey(t.Context(), wallet.GetPublicKeyArgs{IdentityKey: true}, fixtures.DefaultOriginator)
	require.NoError(t, err)

	args.CertifierUrl = certifierServer.URL() // Use the test server URL
	args.Certifier = certifierKey.PublicKey

	// when: Alice acquires a certificate from Bob's certifier server
	actual, err := aliceWallet.AcquireCertificate(t.Context(), args, fixtures.DefaultOriginator)

	// then:
	require.NoError(t, err)
	require.NotNil(t, actual)
	require.Equal(t, args.Type.String(), actual.Type)
}

