package syncrepo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

func TestUpsertCertificateForSync_CreateUpdateAndStaleSkip(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	tOld := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	tNew := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	tStale := time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC)

	isNew, certID, err := repos.UpsertCertificateForSync(t.Context(), &entity.Certificate{
		CreatedAt:          tOld,
		UpdatedAt:          tOld,
		UserID:             user.ID,
		Type:               fixtures.TypeField,
		SerialNumber:       fixtures.SerialNumber,
		Certifier:          fixtures.Certifier,
		Subject:            string(testusers.Alice.PubKey(t)),
		RevocationOutpoint: fixtures.RevocationOutpoint,
		Signature:          fixtures.Signature,
	})
	require.NoError(t, err)
	require.True(t, isNew)
	require.NotZero(t, certID)

	// Happy update: newer updated_at changes subject/signature.
	newSig := "feedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedface"
	isNew, certID2, err := repos.UpsertCertificateForSync(t.Context(), &entity.Certificate{
		CreatedAt:          tOld,
		UpdatedAt:          tNew,
		UserID:             user.ID,
		Type:               fixtures.TypeField,
		SerialNumber:       fixtures.SerialNumber,
		Certifier:          fixtures.Certifier,
		Subject:            "updated-subject",
		RevocationOutpoint: fixtures.RevocationOutpoint,
		Signature:          newSig,
	})
	require.NoError(t, err)
	require.False(t, isNew)
	require.Equal(t, certID, certID2)

	var got models.Certificate
	require.NoError(t, d.DB.Unscoped().First(&got, certID).Error)
	require.Equal(t, "updated-subject", got.Subject)
	require.Equal(t, newSig, got.Signature)
	require.False(t, got.DeletedAt.Valid)

	// Stale skip: older updated_at must not regress.
	isNew, _, err = repos.UpsertCertificateForSync(t.Context(), &entity.Certificate{
		CreatedAt:          tOld,
		UpdatedAt:          tStale,
		UserID:             user.ID,
		Type:               fixtures.TypeField,
		SerialNumber:       fixtures.SerialNumber,
		Certifier:          fixtures.Certifier,
		Subject:            "stale-subject",
		RevocationOutpoint: fixtures.RevocationOutpoint,
		Signature:          "stale",
	})
	require.NoError(t, err)
	require.False(t, isNew)

	require.NoError(t, d.DB.Unscoped().First(&got, certID).Error)
	require.Equal(t, "updated-subject", got.Subject)
	require.Equal(t, newSig, got.Signature)

	// Soft-delete via IsDeleted with newer timestamp.
	tDel := time.Date(2026, 4, 23, 14, 0, 0, 0, time.UTC)
	_, _, err = repos.UpsertCertificateForSync(t.Context(), &entity.Certificate{
		CreatedAt:          tOld,
		UpdatedAt:          tDel,
		UserID:             user.ID,
		Type:               fixtures.TypeField,
		SerialNumber:       fixtures.SerialNumber,
		Certifier:          fixtures.Certifier,
		Subject:            "updated-subject",
		RevocationOutpoint: fixtures.RevocationOutpoint,
		Signature:          newSig,
		IsDeleted:          true,
	})
	require.NoError(t, err)

	require.NoError(t, d.DB.Unscoped().First(&got, certID).Error)
	require.True(t, got.DeletedAt.Valid)

	// Find for sync should return the soft-deleted certificate with IsDeleted=true.
	rows, err := repos.FindCertificatesForSync(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.True(t, rows[0].IsDeleted)
	require.Equal(t, certID, rows[0].CertificateID)
}

func TestUpsertCertificateFieldForSync_CreateUpdateAndStaleSkip(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	tOld := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	tNew := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	tStale := time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC)

	_, certID, err := repos.UpsertCertificateForSync(t.Context(), &entity.Certificate{
		CreatedAt:          tOld,
		UpdatedAt:          tOld,
		UserID:             user.ID,
		Type:               fixtures.TypeField,
		SerialNumber:       fixtures.SerialNumber,
		Certifier:          fixtures.Certifier,
		Subject:            string(testusers.Alice.PubKey(t)),
		RevocationOutpoint: fixtures.RevocationOutpoint,
		Signature:          fixtures.Signature,
	})
	require.NoError(t, err)

	isNew, err := repos.UpsertCertificateFieldForSync(t.Context(), &entity.CertificateField{
		CreatedAt:     tOld,
		UpdatedAt:     tOld,
		UserID:        user.ID,
		CertificateID: certID,
		FieldName:     "exampleField",
		FieldValue:    "value-old",
		MasterKey:     "master-old",
	})
	require.NoError(t, err)
	require.True(t, isNew)

	isNew, err = repos.UpsertCertificateFieldForSync(t.Context(), &entity.CertificateField{
		CreatedAt:     tOld,
		UpdatedAt:     tNew,
		UserID:        user.ID,
		CertificateID: certID,
		FieldName:     "exampleField",
		FieldValue:    "value-new",
		MasterKey:     "master-new",
	})
	require.NoError(t, err)
	require.False(t, isNew)

	var got models.CertificateField
	require.NoError(t, d.DB.Where("certificate_id = ? AND field_name = ?", certID, "exampleField").First(&got).Error)
	require.Equal(t, "value-new", got.FieldValue)
	require.Equal(t, "master-new", got.MasterKey)

	// Stale skip
	_, err = repos.UpsertCertificateFieldForSync(t.Context(), &entity.CertificateField{
		CreatedAt:     tOld,
		UpdatedAt:     tStale,
		UserID:        user.ID,
		CertificateID: certID,
		FieldName:     "exampleField",
		FieldValue:    "value-stale",
		MasterKey:     "master-stale",
	})
	require.NoError(t, err)

	require.NoError(t, d.DB.Where("certificate_id = ? AND field_name = ?", certID, "exampleField").First(&got).Error)
	require.Equal(t, "value-new", got.FieldValue)

	rows, err := repos.FindCertificateFieldsForSync(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, certID, rows[0].CertificateID)
	require.Equal(t, "exampleField", rows[0].FieldName)
	require.Equal(t, "value-new", rows[0].FieldValue)
}

func TestUpsertCommissionForSync_CreateUpdateAndStaleSkip(t *testing.T) {
	d, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()
	repos := d.CreateRepositories()

	user, err := repos.CreateUser(
		t.Context(), testusers.Alice.IdentityKey(t), "test-storage",
		wdk.BasketConfiguration{Name: defaultBasket, NumberOfDesiredUTXOs: 1, MinimumDesiredUTXOValue: 1000},
	)
	require.NoError(t, err)

	tOld := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	tNew := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	tStale := time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC)

	_, txID, err := repos.UpsertTransactionForSync(t.Context(), &entity.Transaction{
		CreatedAt: tOld, UpdatedAt: tOld,
		UserID: user.ID, Status: wdk.TxStatusCompleted, Reference: "ref-commission-sync",
		IsOutgoing: true, Satoshis: 1000, Description: "commission parent",
	})
	require.NoError(t, err)

	script := []byte{0x76, 0xa9, 0x14}
	isNew, commissionID, err := repos.UpsertCommissionForSync(t.Context(), &entity.Commission{
		CreatedAt:     tOld,
		UpdatedAt:     tOld,
		UserID:        user.ID,
		TransactionID: txID,
		Satoshis:      100,
		KeyOffset:     "offset-1",
		IsRedeemed:    false,
		LockingScript: script,
	})
	require.NoError(t, err)
	require.True(t, isNew)
	require.NotZero(t, commissionID)

	// Happy update: only is_redeemed changes on update path.
	isNew, commissionID2, err := repos.UpsertCommissionForSync(t.Context(), &entity.Commission{
		CreatedAt:     tOld,
		UpdatedAt:     tNew,
		UserID:        user.ID,
		TransactionID: txID,
		Satoshis:      999, // ignored on update
		KeyOffset:     "offset-ignored",
		IsRedeemed:    true,
		LockingScript: []byte{0xff},
	})
	require.NoError(t, err)
	require.False(t, isNew)
	require.Equal(t, commissionID, commissionID2)

	var got models.Commission
	require.NoError(t, d.DB.First(&got, commissionID).Error)
	require.True(t, got.IsRedeemed)
	require.Equal(t, uint64(100), got.Satoshis, "satoshis must not change on update (TS mergeExisting)")
	require.Equal(t, "offset-1", got.KeyOffset)
	require.Equal(t, script, got.LockingScript)

	// Stale skip
	_, _, err = repos.UpsertCommissionForSync(t.Context(), &entity.Commission{
		CreatedAt:     tOld,
		UpdatedAt:     tStale,
		UserID:        user.ID,
		TransactionID: txID,
		Satoshis:      100,
		KeyOffset:     "offset-1",
		IsRedeemed:    false,
		LockingScript: script,
	})
	require.NoError(t, err)

	require.NoError(t, d.DB.First(&got, commissionID).Error)
	require.True(t, got.IsRedeemed, "stale chunk must not un-redeem")

	rows, err := repos.FindCommissionsForSync(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, commissionID, rows[0].CommissionID)
	require.Equal(t, txID, rows[0].TransactionID)
	require.True(t, rows[0].IsRedeemed)
}
