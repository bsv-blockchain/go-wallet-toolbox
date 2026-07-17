package repo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	storageentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TestUserUTXOsUpdate_RejectsReservingAlreadyReservedUTXO encodes the
// CAS-guard added to UserUTXOs.Update: when spec.ReservedByID is set, the
// underlying UPDATE must be qualified with "reserved_by_id IS NULL" so a
// concurrently-reserved row is never silently re-reserved.
//
// given: UserUTXO row for output O already has reserved_by_id set (claimed by
// reservingTx).
// when:  Update is called with spec.ReservedByID pointing at a different
// (new) transaction.
// then:  errors.Is(err, repo.ErrUTXOContention), and the row's reserved_by_id
// is unchanged (still reservingTx).
func TestUserUTXOsUpdate_RejectsReservingAlreadyReservedUTXO(t *testing.T) {
	// given:
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB)
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "utxo-cas-a-user", "test-storage", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	ownerTx := createTestTx(t, repos, ctx, user.ID, "utxo-cas-a-owner-tx")
	reservingTx := createTestTx(t, repos, ctx, user.ID, "utxo-cas-a-reserving-tx")
	newTx := createTestTx(t, repos, ctx, user.ID, "utxo-cas-a-new-tx")

	output := createTestOutput(t, db.DB, user.ID, ownerTx.ID, 0)

	reservedByID := reservingTx.ID
	utxo := models.UserUTXO{
		UserID:             user.ID,
		OutputID:           output.ID,
		BasketName:         wdk.BasketNameForChange,
		Satoshis:           1000,
		EstimatedInputSize: 148,
		UTXOStatus:         wdk.UTXOStatusUnproven,
		ReservedByID:       &reservedByID,
	}
	require.NoError(t, db.DB.Create(&utxo).Error)

	// when:
	newReservation := newTx.ID
	updateErr := repos.Update(ctx, &entity.UserUTXOUpdateSpecification{
		OutputID:     output.ID,
		ReservedByID: &newReservation,
	})

	// then:
	require.Error(t, updateErr)
	require.ErrorIs(t, updateErr, repo.ErrUTXOContention)

	var got models.UserUTXO
	require.NoError(t, db.DB.Where("output_id = ?", output.ID).First(&got).Error)
	require.NotNil(t, got.ReservedByID)
	require.Equal(t, reservedByID, *got.ReservedByID, "row must remain reserved by the original transaction")
}

// TestUserUTXOsUpdate_ReservesUnreservedUTXO covers the happy path of the same
// guard: an unreserved row (reserved_by_id IS NULL) must still be reservable.
//
// given: UserUTXO row for output O with reserved_by_id = NULL.
// when:  Update is called with spec.ReservedByID pointing at a transaction.
// then:  no error, and the row's reserved_by_id is now set to that transaction.
func TestUserUTXOsUpdate_ReservesUnreservedUTXO(t *testing.T) {
	// given:
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB)
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "utxo-cas-b-user", "test-storage", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	ownerTx := createTestTx(t, repos, ctx, user.ID, "utxo-cas-b-owner-tx")
	newTx := createTestTx(t, repos, ctx, user.ID, "utxo-cas-b-new-tx")

	output := createTestOutput(t, db.DB, user.ID, ownerTx.ID, 0)

	utxo := models.UserUTXO{
		UserID:             user.ID,
		OutputID:           output.ID,
		BasketName:         wdk.BasketNameForChange,
		Satoshis:           1000,
		EstimatedInputSize: 148,
		UTXOStatus:         wdk.UTXOStatusUnproven,
		ReservedByID:       nil,
	}
	require.NoError(t, db.DB.Create(&utxo).Error)

	// when:
	newReservation := newTx.ID
	updateErr := repos.Update(ctx, &entity.UserUTXOUpdateSpecification{
		OutputID:     output.ID,
		ReservedByID: &newReservation,
	})

	// then:
	require.NoError(t, updateErr)

	var got models.UserUTXO
	require.NoError(t, db.DB.Where("output_id = ?", output.ID).First(&got).Error)
	require.NotNil(t, got.ReservedByID)
	require.Equal(t, newReservation, *got.ReservedByID)
}

// TestUserUTXOsUpdate_OtherFieldsUnaffectedByGuard confirms the guard is scoped
// strictly to spec.ReservedByID: updates that only touch other fields must
// behave exactly as before, even on an already-reserved row, and must not
// disturb reserved_by_id.
//
// given: UserUTXO row for output O already reserved by reservingTx.
// when:  Update is called with only Status set (spec.ReservedByID == nil).
// then:  no error, the row's status is updated, and reserved_by_id is untouched.
func TestUserUTXOsUpdate_OtherFieldsUnaffectedByGuard(t *testing.T) {
	// given:
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB)
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "utxo-cas-c-user", "test-storage", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	ownerTx := createTestTx(t, repos, ctx, user.ID, "utxo-cas-c-owner-tx")
	reservingTx := createTestTx(t, repos, ctx, user.ID, "utxo-cas-c-reserving-tx")

	output := createTestOutput(t, db.DB, user.ID, ownerTx.ID, 0)

	reservedByID := reservingTx.ID
	utxo := models.UserUTXO{
		UserID:             user.ID,
		OutputID:           output.ID,
		BasketName:         wdk.BasketNameForChange,
		Satoshis:           1000,
		EstimatedInputSize: 148,
		UTXOStatus:         wdk.UTXOStatusUnproven,
		ReservedByID:       &reservedByID,
	}
	require.NoError(t, db.DB.Create(&utxo).Error)

	// when:
	newStatus := wdk.UTXOStatusMined
	updateErr := repos.Update(ctx, &entity.UserUTXOUpdateSpecification{
		OutputID: output.ID,
		Status:   &newStatus,
	})

	// then:
	require.NoError(t, updateErr)

	var got models.UserUTXO
	require.NoError(t, db.DB.Where("output_id = ?", output.ID).First(&got).Error)
	require.Equal(t, newStatus, got.UTXOStatus)
	require.NotNil(t, got.ReservedByID)
	require.Equal(t, reservedByID, *got.ReservedByID, "reserved_by_id must be untouched by an update that doesn't target it")
}

// createTestTx creates and returns a minimal transaction owned by userID, used
// as an FK parent for outputs and user_utxos.reserved_by_id in these tests.
func createTestTx(t *testing.T, repos *repo.Repositories, ctx context.Context, userID int, reference string) *entity.Transaction {
	t.Helper()
	require.NoError(t, repos.CreateTransaction(ctx, &storageentity.NewTx{
		UserID:    userID,
		Status:    wdk.TxStatusCompleted,
		Reference: reference,
		Version:   1,
	}))
	tx, err := repos.FindTransactionByReference(ctx, userID, reference)
	require.NoError(t, err)
	require.NotNil(t, tx)
	return tx
}

// createTestOutput creates a minimal, spendable output owned by userID under
// transactionID, used as the FK parent for a bsv_user_utxos row.
func createTestOutput(t *testing.T, db *gorm.DB, userID int, transactionID uint, vout uint32) *models.Output {
	t.Helper()
	output := &models.Output{
		UserID:        userID,
		TransactionID: transactionID,
		Vout:          vout,
		Satoshis:      1000,
		Spendable:     true,
		Change:        false,
		ProvidedBy:    string(wdk.ProvidedByYou),
		Type:          "P2PKH",
	}
	require.NoError(t, db.Create(output).Error)
	return output
}
