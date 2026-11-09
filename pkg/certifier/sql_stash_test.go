package certifier

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSQLStashSaveAndFind(t *testing.T) {
	stash := newTestStash(t)
	cert := testIssuedCertificate("serial-1", "subject-1", "txid.0")

	err := stash.Save(t.Context(), cert)

	require.NoError(t, err)
	require.NotZero(t, cert.ID)

	bySerial, err := stash.FindBySerialNumber(t.Context(), "serial-1")
	require.NoError(t, err)
	requireSameIssuedCertificate(t, cert, bySerial)

	byOutpoint, err := stash.FindByRevocationOutpoint(t.Context(), "txid.0")
	require.NoError(t, err)
	requireSameIssuedCertificate(t, cert, byOutpoint)

	bySubject, err := stash.FindBySubject(t.Context(), "subject-1")
	require.NoError(t, err)
	require.Len(t, bySubject, 1)
	requireSameIssuedCertificate(t, cert, bySubject[0])
}

func TestSQLStashFindBySubjectReturnsAllCertificatesOfSubject(t *testing.T) {
	stash := newTestStash(t)
	require.NoError(t, stash.Save(t.Context(), testIssuedCertificate("serial-1", "subject-1", "txid1.0")))
	require.NoError(t, stash.Save(t.Context(), testIssuedCertificate("serial-2", "subject-2", "txid2.0")))
	require.NoError(t, stash.Save(t.Context(), testIssuedCertificate("serial-3", "subject-1", "txid3.0")))

	certs, err := stash.FindBySubject(t.Context(), "subject-1")

	require.NoError(t, err)
	require.Len(t, certs, 2)
	require.Equal(t, "serial-1", certs[0].SerialNumber)
	require.Equal(t, "serial-3", certs[1].SerialNumber)
}

func TestSQLStashFindReturnsNotFound(t *testing.T) {
	stash := newTestStash(t)

	_, err := stash.FindBySerialNumber(t.Context(), "missing")
	require.ErrorIs(t, err, ErrIssuedCertificateNotFound)

	_, err = stash.FindByRevocationOutpoint(t.Context(), "missing.0")
	require.ErrorIs(t, err, ErrIssuedCertificateNotFound)

	certs, err := stash.FindBySubject(t.Context(), "missing")
	require.NoError(t, err)
	require.Empty(t, certs)
}

func TestSQLStashRejectsDuplicateSerialNumber(t *testing.T) {
	stash := newTestStash(t)
	require.NoError(t, stash.Save(t.Context(), testIssuedCertificate("serial-1", "subject-1", "txid1.0")))

	err := stash.Save(t.Context(), testIssuedCertificate("serial-1", "subject-2", "txid2.0"))

	require.Error(t, err)
	_, err = stash.FindByRevocationOutpoint(t.Context(), "txid2.0")
	require.ErrorIs(t, err, ErrIssuedCertificateNotFound)
}

func TestSQLStashSaveLeavesNoOrphanRowsWhenFieldInsertFails(t *testing.T) {
	stash := newTestStash(t)
	err := stash.db.Callback().Create().Before("gorm:create").Register("test:fail_fields", func(db *gorm.DB) {
		if db.Statement.Table == (issuedCertificateFieldModel{}).TableName() {
			_ = db.AddError(errors.New("field insert failed"))
		}
	})
	require.NoError(t, err)

	err = stash.Save(t.Context(), testIssuedCertificate("serial-1", "subject-1", "txid.0"))

	require.ErrorContains(t, err, "field insert failed")
	var certCount, fieldCount int64
	require.NoError(t, stash.db.Model(&issuedCertificateModel{}).Count(&certCount).Error)
	require.NoError(t, stash.db.Model(&issuedCertificateFieldModel{}).Count(&fieldCount).Error)
	require.Zero(t, certCount)
	require.Zero(t, fieldCount)
}

func TestNewSQLiteStashInMemoryDatabasesAreSeparate(t *testing.T) {
	first := newTestStash(t)
	second := newTestStash(t)
	require.NoError(t, first.Save(t.Context(), testIssuedCertificate("serial-1", "subject-1", "txid.0")))

	_, err := second.FindBySerialNumber(t.Context(), "serial-1")

	require.ErrorIs(t, err, ErrIssuedCertificateNotFound)
}

func newTestStash(t *testing.T) *SQLStash {
	t.Helper()
	stash, err := NewSQLiteStash("")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, stash.Close())
	})
	return stash
}

func testIssuedCertificate(serialNumber, subject, outpoint string) *IssuedCertificate {
	return &IssuedCertificate{
		Type:               "dHlwZQ==",
		SerialNumber:       serialNumber,
		Subject:            subject,
		Certifier:          "certifier",
		RevocationOutpoint: outpoint,
		Signature:          "3044",
		Fields: []IssuedCertificateField{
			{Name: "email", Value: "ZW5jcnlwdGVkLWVtYWls", MasterKey: "a2V5LWVtYWls"},
			{Name: "name", Value: "ZW5jcnlwdGVkLW5hbWU=", MasterKey: "a2V5LW5hbWU="},
		},
		RevocationTx: []byte{1, 2, 3},
	}
}

func requireSameIssuedCertificate(t *testing.T, expected, actual *IssuedCertificate) {
	t.Helper()
	require.Equal(t, expected.ID, actual.ID)
	require.Equal(t, expected.Type, actual.Type)
	require.Equal(t, expected.SerialNumber, actual.SerialNumber)
	require.Equal(t, expected.Subject, actual.Subject)
	require.Equal(t, expected.Certifier, actual.Certifier)
	require.Equal(t, expected.RevocationOutpoint, actual.RevocationOutpoint)
	require.Equal(t, expected.Signature, actual.Signature)
	require.Equal(t, expected.Fields, actual.Fields)
	require.Equal(t, expected.RevocationTx, actual.RevocationTx)
	require.False(t, actual.CreatedAt.IsZero())
}
