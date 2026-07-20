package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TestGetSyncChunk_CertificatesAndFields verifies getSyncChunk returns certificate
// entities that were previously inserted (issue #850).
func TestGetSyncChunk_CertificatesAndFields(t *testing.T) {
	given, then, cleanup := testabilities.NewSync(t)
	defer cleanup()

	givenProvider := given.Provider()
	activeStorage := givenProvider.GORM()

	// Seed a certificate with one field for Alice.
	certToInsert := fixtures.DefaultInsertCertAuth(testusers.Alice.ID, primitives.PubKeyHex(testusers.Alice.PubKey(t)))
	certID, err := activeStorage.InsertCertificateAuth(t.Context(), testusers.Alice.AuthID(), certToInsert)
	require.NoError(t, err)
	require.NotZero(t, certID)

	args := given.RequestSyncChunk(testusers.Alice).Args()

	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)
	thenChunk := then.Chunk(chunk).WithoutError(err)
	thenChunk.WithGeneralInfo(&args)

	require.Len(t, chunk.Certificates, 1)
	require.Equal(t, certID, chunk.Certificates[0].CertificateID)
	require.Equal(t, fixtures.SerialNumber, string(chunk.Certificates[0].SerialNumber))
	require.Equal(t, fixtures.Certifier, string(chunk.Certificates[0].Certifier))
	require.False(t, chunk.Certificates[0].IsDeleted)

	require.Len(t, chunk.CertificateFields, 1)
	require.Equal(t, certID, chunk.CertificateFields[0].CertificateID)
	require.Equal(t, "exampleField", chunk.CertificateFields[0].FieldName)
	require.Equal(t, "exampleValue", chunk.CertificateFields[0].FieldValue)
}

// TestGetSyncChunk_CertificateAfterRelinquish verifies that relinquish bumps
// updated_at so a since-filtered chunk includes the soft-deleted certificate.
func TestGetSyncChunk_CertificateAfterRelinquish(t *testing.T) {
	given, _, cleanup := testabilities.NewSync(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	certToInsert := fixtures.DefaultInsertCertAuth(testusers.Alice.ID, primitives.PubKeyHex(testusers.Alice.PubKey(t)))
	certID, err := activeStorage.InsertCertificateAuth(t.Context(), testusers.Alice.AuthID(), certToInsert)
	require.NoError(t, err)
	require.NotZero(t, certID)

	// Capture a "when" after create so the subsequent since-filter only sees the delete.
	afterCreate := time.Now().UTC()
	time.Sleep(2 * time.Millisecond)

	err = activeStorage.RelinquishCertificate(t.Context(), testusers.Alice.AuthID(), wdk.RelinquishCertificateArgs{
		Type:         fixtures.TypeField,
		SerialNumber: fixtures.SerialNumber,
		Certifier:    fixtures.Certifier,
	})
	require.NoError(t, err)

	args := given.RequestSyncChunk(testusers.Alice).WithSince(afterCreate).Args()
	chunk, err := activeStorage.GetSyncChunk(t.Context(), args)
	require.NoError(t, err)
	require.Len(t, chunk.Certificates, 1)
	require.Equal(t, certID, chunk.Certificates[0].CertificateID)
	require.True(t, chunk.Certificates[0].IsDeleted)
}

// TestSyncProcess_CertificatesAndFields verifies certificates and fields survive
// SyncToWriter end-to-end (processSyncChunk path of issue #850). Soft-delete
// merge is covered by syncrepo unit tests (TestUpsertCertificateForSync_*).
func TestSyncProcess_CertificatesAndFields(t *testing.T) {
	givenSourceDB, cleanup := testabilities.GivenSyncFixture(t)
	defer cleanup()

	sourceProvider := givenSourceDB.Provider().GORM()
	sourceStorageManager := givenSourceDB.StorageManagerForUser(testusers.Alice, sourceProvider)

	// Seed certificate on source.
	certToInsert := fixtures.DefaultInsertCertAuth(testusers.Alice.ID, primitives.PubKeyHex(testusers.Alice.PubKey(t)))
	sourceCertID, err := sourceProvider.InsertCertificateAuth(t.Context(), testusers.Alice.AuthID(), certToInsert)
	require.NoError(t, err)
	require.NotZero(t, sourceCertID)

	// Backup storage (empty).
	givenBackupDB, cleanup := testabilities.GivenCustomStorage(t, fixtures.SecondStorageServerPrivKey, fixtures.SecondStorageName)
	defer cleanup()
	backupProvider := givenBackupDB.Provider().GORMWithCleanDatabase()

	_, err = sourceStorageManager.MakeAvailable(t.Context())
	require.NoError(t, err)

	inserts, updates, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)
	require.NoError(t, err)
	assert.Positive(t, inserts)
	_ = updates

	// Verify certificate landed on backup via ListCertificates.
	result, err := backupProvider.ListCertificates(t.Context(), testusers.Alice.AuthID(), wdk.ListCertificatesArgs{})
	require.NoError(t, err)
	require.Equal(t, primitives.PositiveInteger(1), result.TotalCertificates)
	require.Len(t, result.Certificates, 1)
	require.Equal(t, fixtures.SerialNumber, string(result.Certificates[0].SerialNumber))
	require.Equal(t, fixtures.Certifier, string(result.Certificates[0].Certifier))
	require.Equal(t, "exampleValue", result.Certificates[0].Fields[primitives.StringUnder50Bytes("exampleField")])

	// Second sync should be a no-op (or only minor updates), not re-insert the cert.
	inserts2, _, err := sourceStorageManager.SyncToWriter(t.Context(), backupProvider)
	require.NoError(t, err)
	assert.Equal(t, 0, inserts2)
}

// TestProcessSyncChunk_CertOnlyDoesNotMarkDone guards the emptyChunk fix: a chunk
// that only carries certificates must not be treated as the terminal empty chunk.
func TestProcessSyncChunk_CertOnlyDoesNotMarkDone(t *testing.T) {
	given, _, cleanup := testabilities.NewSync(t)
	defer cleanup()

	activeStorage := given.Provider().GORM()

	// Ensure Alice has a sync state for the reader (from) storage identity key.
	_, err := activeStorage.FindOrInsertSyncStateAuth(
		t.Context(),
		testusers.Alice.AuthID(),
		fixtures.SecondStorageIdentityKey,
		fixtures.SecondStorageName,
	)
	require.NoError(t, err)

	now := time.Now().UTC()
	chunk := wdk.NewSyncChunk(fixtures.SecondStorageIdentityKey, fixtures.StorageIdentityKey, testusers.Alice.IdentityKey(t))
	chunk.Certificates = append(chunk.Certificates, &wdk.TableCertificate{
		CreatedAt:          now,
		UpdatedAt:          now,
		CertificateID:      1,
		UserID:             testusers.Alice.ID,
		Type:               primitives.Base64String(fixtures.TypeField),
		SerialNumber:       primitives.Base64String(fixtures.SerialNumber),
		Certifier:          primitives.PubKeyHex(fixtures.Certifier),
		Subject:            primitives.PubKeyHex(testusers.Alice.PubKey(t)),
		RevocationOutpoint: primitives.OutpointString(fixtures.RevocationOutpoint),
		Signature:          primitives.HexString(fixtures.Signature),
		IsDeleted:          false,
	})

	args := fixtures.DefaultRequestSyncChunkArgs(
		testusers.Alice.IdentityKey(t),
		fixtures.SecondStorageIdentityKey,
		fixtures.StorageIdentityKey,
	)

	result, err := activeStorage.ProcessSyncChunk(t.Context(), args, chunk)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Done, "cert-only chunk must not mark sync done")
	require.Positive(t, result.Inserts)
}
