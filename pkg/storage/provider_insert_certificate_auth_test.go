package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

func TestInsertCertificateAuth(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	activeStorage := given.Provider().GORM()

	// and:
	expectedResult := &wdk.ListCertificatesResult{
		Certificates:      make([]*wdk.CertificateResult, 0),
		TotalCertificates: primitives.PositiveInteger(0),
	}

	t.Run("should insert a certificate for Alice", func(t *testing.T) {
		// given:
		certToInsert := fixtures.DefaultInsertCertAuth(testusers.Alice.ID, primitives.PubKeyHex(testusers.Alice.PubKey(t)))
		// and:
		expectedResult.TotalCertificates = primitives.PositiveInteger(1)
		expectedResult.Certificates = []*wdk.CertificateResult{{
			Verifier: "",
			WalletCertificate: wdk.WalletCertificate{
				Type:               fixtures.TypeField,
				Subject:            primitives.PubKeyHex(testusers.Alice.PublicKey(t).ToDERHex()),
				SerialNumber:       fixtures.SerialNumber,
				Certifier:          fixtures.Certifier,
				RevocationOutpoint: fixtures.RevocationOutpoint,
				Signature:          fixtures.Signature,
				Fields: map[primitives.StringUnder50Bytes]string{
					"exampleField": "exampleValue",
				},
			},
			Keyring: map[primitives.StringUnder50Bytes]primitives.Base64String{
				"exampleField": "exampleValue",
			},
		}}

		// when: insert certificate for Alice
		_, err := activeStorage.InsertCertificateAuth(t.Context(), testusers.Alice.AuthID(), certToInsert)

		// then:
		require.NoError(t, err)

		// when: listing certificates
		certs, err := activeStorage.ListCertificates(t.Context(), testusers.Alice.AuthID(), wdk.ListCertificatesArgs{})

		// then:
		require.NoError(t, err)
		require.Equal(t, expectedResult, certs)
	})

	t.Run("should insert a certificate for Bob", func(t *testing.T) {
		// given:
		bobPubKey := primitives.PubKeyHex(testusers.Bob.PubKey(t))
		certToInsert := fixtures.DefaultInsertCertAuth(testusers.Bob.ID, bobPubKey)
		// and:
		expectedResult.TotalCertificates = primitives.PositiveInteger(2)
		expectedResult.Certificates = []*wdk.CertificateResult{{
			Verifier: "",
			WalletCertificate: wdk.WalletCertificate{
				Type:               "ZXhhbXBsZVR5cGUy",
				Subject:            bobPubKey,
				SerialNumber:       fixtures.SerialNumber,
				Certifier:          fixtures.Certifier,
				RevocationOutpoint: fixtures.RevocationOutpoint,
				Signature:          fixtures.Signature,
				Fields: map[primitives.StringUnder50Bytes]string{
					"exampleField": "exampleValue",
				},
			},
			Keyring: map[primitives.StringUnder50Bytes]primitives.Base64String{
				"exampleField": "exampleValue",
			},
		}, {
			Verifier: "",
			WalletCertificate: wdk.WalletCertificate{
				Type:               fixtures.TypeField,
				Subject:            bobPubKey,
				SerialNumber:       fixtures.SerialNumber,
				Certifier:          fixtures.Certifier,
				RevocationOutpoint: fixtures.RevocationOutpoint,
				Signature:          fixtures.Signature,
				Fields: map[primitives.StringUnder50Bytes]string{
					"exampleField": "exampleValue",
				},
			},
			Keyring: map[primitives.StringUnder50Bytes]primitives.Base64String{
				"exampleField": "exampleValue",
			},
		}}

		// when: insert certificate for Bob
		_, err := activeStorage.InsertCertificateAuth(t.Context(), testusers.Bob.AuthID(), certToInsert)

		// then:
		require.NoError(t, err)

		// when:
		certToInsert.Type = "ZXhhbXBsZVR5cGUy"
		_, err = activeStorage.InsertCertificateAuth(t.Context(), testusers.Bob.AuthID(), certToInsert)

		// then:
		require.NoError(t, err)

		// when: listing certificates
		certs, err := activeStorage.ListCertificates(t.Context(), testusers.Bob.AuthID(), wdk.ListCertificatesArgs{})

		// then:
		require.NoError(t, err)
		assert.Equal(t, expectedResult.TotalCertificates, primitives.PositiveInteger(2))
		require.ElementsMatch(t, certs.Certificates, expectedResult.Certificates)
	})

	t.Run("should delete a certificate for Bob", func(t *testing.T) {
		// given:
		bobPubKey := primitives.PubKeyHex(testusers.Bob.PubKey(t))
		certs, err := activeStorage.ListCertificates(t.Context(), testusers.Bob.AuthID(), wdk.ListCertificatesArgs{})
		require.NoError(t, err)
		require.Equal(t, primitives.PositiveInteger(2), certs.TotalCertificates)

		// and:
		expectedResult.TotalCertificates = primitives.PositiveInteger(1)
		expectedResult.Certificates = []*wdk.CertificateResult{{
			Verifier: "",
			WalletCertificate: wdk.WalletCertificate{
				Type:               fixtures.TypeField,
				Subject:            bobPubKey,
				SerialNumber:       fixtures.SerialNumber,
				Certifier:          fixtures.Certifier,
				RevocationOutpoint: fixtures.RevocationOutpoint,
				Signature:          fixtures.Signature,
				Fields: map[primitives.StringUnder50Bytes]string{
					"exampleField": "exampleValue",
				},
			},
			Keyring: map[primitives.StringUnder50Bytes]primitives.Base64String{
				"exampleField": "exampleValue",
			},
		}}

		// when:
		err = activeStorage.RelinquishCertificate(t.Context(), testusers.Bob.AuthID(), wdk.RelinquishCertificateArgs{
			Type:         "ZXhhbXBsZVR5cGUy",
			SerialNumber: fixtures.SerialNumber,
			Certifier:    fixtures.Certifier,
		})

		// then:
		require.NoError(t, err)

		// when: list certificates
		certs, err = activeStorage.ListCertificates(t.Context(), testusers.Bob.AuthID(), wdk.ListCertificatesArgs{})

		// then:
		require.NoError(t, err)
		assert.Equal(t, expectedResult.TotalCertificates, primitives.PositiveInteger(1))
		require.ElementsMatch(t, certs.Certificates, expectedResult.Certificates)
	})
}

func TestInsertCertificateAuthFailure(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	activeStorage := given.Provider().GORM()

	t.Run("should fail to insert a certificate when no UserID provided in auth and when certificate UserID is different than authID", func(t *testing.T) {
		// when:
		_, err := activeStorage.InsertCertificateAuth(t.Context(), wdk.AuthID{}, &wdk.TableCertificateX{})

		// then:
		require.ErrorContains(t, err, "access is denied due to an authorization error")

		// and when:
		_, err = activeStorage.InsertCertificateAuth(t.Context(), testusers.Alice.AuthID(), &wdk.TableCertificateX{
			TableCertificate: wdk.TableCertificate{
				UserID: testusers.Bob.ID,
			},
		})

		// then:
		require.ErrorContains(t, err, "access is denied due to an authorization error")
	})

	t.Run("should fail to relinquish a certificate when no UserID provided in auth", func(t *testing.T) {
		// when:
		err := activeStorage.RelinquishCertificate(t.Context(), wdk.AuthID{}, wdk.RelinquishCertificateArgs{})

		// then:
		require.ErrorContains(t, err, "access is denied due to an authorization error")
	})

	t.Run("should fail to list certificate when no UserID provided in auth", func(t *testing.T) {
		// when:
		_, err := activeStorage.ListCertificates(t.Context(), wdk.AuthID{}, wdk.ListCertificatesArgs{})

		// then:
		require.ErrorContains(t, err, "access is denied due to an authorization error")
	})

	t.Run("should fail to delete certificate when no cert is found", func(t *testing.T) {
		// when:
		err := activeStorage.RelinquishCertificate(t.Context(), testusers.Alice.AuthID(), wdk.RelinquishCertificateArgs{
			Type:         "bm90LXR5cGU=",
			SerialNumber: fixtures.SerialNumber,
			Certifier:    fixtures.Certifier,
		})

		// then:
		require.ErrorContains(t, err, "failed to delete certificate model: certificate not found")
	})
}

func TestListCertificates(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()

	// and:
	activeStorage := given.Provider().GORM()

	t.Run("should insert 3 certificates for Alice", func(t *testing.T) {
		// given:
		certToInsert := fixtures.DefaultInsertCertAuth(testusers.Alice.ID, primitives.PubKeyHex(testusers.Alice.PubKey(t)))
		// when: insert 1st certificate for Bob
		_, err := activeStorage.InsertCertificateAuth(t.Context(), testusers.Alice.AuthID(), certToInsert)

		// then:
		require.NoError(t, err)

		// when: update 2nd cert type and insert
		certToInsert.Type = "ZXhhbXBsZVR5cGUy"
		_, err = activeStorage.InsertCertificateAuth(t.Context(), testusers.Alice.AuthID(), certToInsert)

		// then:
		require.NoError(t, err)

		// when: update 3nd cert type and insert
		certToInsert.Type = "ZXhhbXBsZVR5cGUz"
		_, err = activeStorage.InsertCertificateAuth(t.Context(), testusers.Alice.AuthID(), certToInsert)

		// then:
		require.NoError(t, err)

		// when: listing certificates with limit 1
		certs, err := activeStorage.ListCertificates(t.Context(), testusers.Alice.AuthID(), wdk.ListCertificatesArgs{
			Limit: primitives.PositiveIntegerDefault10Max10000(1),
		})

		// then:
		require.NoError(t, err)
		assert.Equal(t, primitives.PositiveInteger(3), certs.TotalCertificates)
		require.Len(t, certs.Certificates, 1)

		// when: listing certificates with limit 2
		certs, err = activeStorage.ListCertificates(t.Context(), testusers.Alice.AuthID(), wdk.ListCertificatesArgs{
			Limit: primitives.PositiveIntegerDefault10Max10000(2),
		})

		// then:
		require.NoError(t, err)
		assert.Equal(t, primitives.PositiveInteger(3), certs.TotalCertificates)
		require.Len(t, certs.Certificates, 2)

		// when: listing certificates with limit 1 and offset 2
		certs, err = activeStorage.ListCertificates(t.Context(), testusers.Alice.AuthID(), wdk.ListCertificatesArgs{
			Offset: primitives.PositiveInteger(2),
		})

		// then:
		require.NoError(t, err)
		assert.Equal(t, primitives.PositiveInteger(3), certs.TotalCertificates)
		require.Len(t, certs.Certificates, 1)
	})
}
