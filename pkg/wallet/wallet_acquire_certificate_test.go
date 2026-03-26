package wallet_test

import (
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	certs_testabilities "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/testabilities"
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

	// and: create a certifier server wallet (the server that will issue certificates)
	certifierWallet := given.BobWalletWithStorage(s.StorageType) // Bob acts as certifier

	// and: fund the certifier wallet (needed for creating revocation transaction)
	given.Faucet(certifierWallet).TopUp(1000)

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

	// then: no error occurred
	require.NoError(t, err)
	require.NotNil(t, actual)

	// and: get Alice's identity key for validation
	aliceKey, err := aliceWallet.GetPublicKey(t.Context(), wallet.GetPublicKeyArgs{IdentityKey: true}, fixtures.DefaultOriginator)
	require.NoError(t, err)

	// and: verify certificate fields
	require.Equal(t, args.Type, actual.Type, "certificate type should match requested type")
	require.Equal(t, aliceKey.PublicKey.ToDERHex(), actual.Subject.ToDERHex(), "certificate subject should be Alice's identity key")
	require.Equal(t, certifierKey.PublicKey.ToDERHex(), actual.Certifier.ToDERHex(), "certificate certifier should be Bob's identity key")
	require.NotNil(t, actual.Signature, "certificate should have a valid signature")
	require.NotNil(t, actual.RevocationOutpoint, "certificate should have a revocation outpoint")
	require.Len(t, actual.SerialNumber, 32, "serial number should be 32 bytes")

	// and: verify fields match what was requested
	require.Len(t, actual.Fields, len(args.Fields), "certificate should have same number of fields as requested")
	for fieldName, fieldValue := range args.Fields {
		actualValue, exists := actual.Fields[fieldName]
		require.True(t, exists, "certificate should contain field: %s", fieldName)
		require.Equal(t, fieldValue, actualValue, "field %s should have correct value", fieldName)
	}

	// and: verify certificate was stored in database
	listResult, err := aliceWallet.ListCertificates(t.Context(), wallet.ListCertificatesArgs{
		Certifiers: []*ec.PublicKey{certifierKey.PublicKey},
		Limit:      to.Ptr(uint32(10)),
	}, fixtures.DefaultOriginator)
	require.NoError(t, err)
	require.NotNil(t, listResult)
	require.Equal(t, uint32(1), listResult.TotalCertificates, "should have 1 certificate stored")
	require.Len(t, listResult.Certificates, 1, "should return 1 certificate")

	// and: verify stored certificate matches returned certificate
	storedCert := listResult.Certificates[0].Certificate
	require.Equal(t, actual.Type, storedCert.Type)
	require.Equal(t, actual.SerialNumber, storedCert.SerialNumber)
	require.Equal(t, actual.Subject.ToDERHex(), storedCert.Subject.ToDERHex())
	require.Equal(t, actual.Certifier.ToDERHex(), storedCert.Certifier.ToDERHex())
}
