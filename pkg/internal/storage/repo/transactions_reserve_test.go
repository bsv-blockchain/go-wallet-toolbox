package repo_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	storageentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TestReserveUTXOs_RejectsAlreadyReservedUTXO encodes the change-path (funder pool)
// double-claim guard from Decision Record v1 (W1-1 / T4's CAS-guarded UserUTXOs.Update)
// at the repo level, mirroring TestMarkReservedOutputsAsNotSpendable_RejectsAlreadyClaimedOutput
// but for the reserveUTXOs (bsv_user_utxos.reserved_by_id) path instead of
// markReservedOutputsAsNotSpendable (bsv_outputs.spent_by).
//
// given: change UTXO O owned by user U, already reserved (reserved_by_id set) by tx A.
// when:  CreateTransactionInTx for tx B lists O's output id in ReservedOutputIDs.
// then:  errors.Is(err, repo.ErrUTXOContention), and tx B's own insert is rolled
// back (no bsv_transactions row for B exists afterward).
func TestReserveUTXOs_RejectsAlreadyReservedUTXO(t *testing.T) {
	// given:
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB)
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "reserve-contention-user", "test-storage", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	// owner: the transaction that originally created the change output O.
	require.NoError(t, repos.CreateTransaction(ctx, &storageentity.NewTx{
		UserID:    user.ID,
		Status:    wdk.TxStatusCompleted,
		Reference: "reserve-contention-owner-tx",
		Version:   1,
	}))
	ownerTx, err := repos.FindTransactionByReference(ctx, user.ID, "reserve-contention-owner-tx")
	require.NoError(t, err)
	require.NotNil(t, ownerTx)

	// reserver: the transaction that already reserved (reserved_by_id) the change UTXO O.
	require.NoError(t, repos.CreateTransaction(ctx, &storageentity.NewTx{
		UserID:    user.ID,
		Status:    wdk.TxStatusUnsigned,
		Reference: "reserve-contention-reserver-tx",
		Version:   1,
	}))
	reserverTx, err := repos.FindTransactionByReference(ctx, user.ID, "reserve-contention-reserver-tx")
	require.NoError(t, err)
	require.NotNil(t, reserverTx)

	// O: a change output owned by ownerTx, already reserved by reserverTx. This
	// mirrors the shape produced by makeNewOutput for a spendable change output
	// (Spendable=true, Change=true, BasketName set), plus its UserUTXO row.
	basketName := wdk.BasketNameForChange
	changeOutput := models.Output{
		UserID:        user.ID,
		TransactionID: ownerTx.ID,
		Vout:          0,
		Satoshis:      20_000,
		Spendable:     true,
		Change:        true,
		ProvidedBy:    string(wdk.ProvidedByStorage),
		Purpose:       wdk.ChangePurpose,
		Type:          string(wdk.OutputTypeP2PKH),
		BasketName:    &basketName,
	}
	require.NoError(t, db.DB.Create(&changeOutput).Error)

	utxo := models.UserUTXO{
		UserID:             user.ID,
		OutputID:           changeOutput.ID,
		Satoshis:           20_000,
		EstimatedInputSize: txutils.P2PKHEstimatedInputSize,
		BasketName:         basketName,
		UTXOStatus:         wdk.UTXOStatusUnproven,
		ReservedByID:       &reserverTx.ID,
	}
	require.NoError(t, db.DB.Create(&utxo).Error)

	// when: tx C tries to reserve the already-reserved change UTXO O via ReservedOutputIDs.
	newTxC := &storageentity.NewTx{
		UserID:            user.ID,
		Status:            wdk.TxStatusUnsigned,
		Reference:         "reserve-contention-tx-c",
		Version:           1,
		ReservedOutputIDs: []uint{changeOutput.ID},
	}

	createErr := db.DB.Transaction(func(tx *gorm.DB) error {
		return repos.CreateTransactionInTx(ctx, tx, newTxC)
	})

	// then:
	require.Error(t, createErr)
	require.ErrorIs(t, createErr, repo.ErrUTXOContention)

	var count int64
	require.NoError(t, db.DB.Model(&models.Transaction{}).
		Where("reference = ?", newTxC.Reference).
		Count(&count).Error)
	require.Zerof(t, count, "tx C's transaction row should have been rolled back, found %d", count)
}
