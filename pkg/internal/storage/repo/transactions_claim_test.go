package repo_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	storageentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TestMarkReservedOutputsAsNotSpendable_RejectsAlreadyClaimedOutput encodes the
// provided-input (KnownOutputIDs) double-claim from Decision Record v1 (W1-1) at
// the repo level: an output whose spent_by column is already set (claimed by
// some earlier transaction) must not be silently "re-claimed" by a second
// createTransactionInTx call.
//
// given: output O owned by user U, already claimed (spent_by set) by tx A.
// when:  CreateTransactionInTx for tx B lists O's id in SpentOutputIDs.
// then:  errors.Is(err, repo.ErrUTXOContention), and tx B's own insert is
// rolled back (no bsv_transactions row for B exists afterward).
func TestMarkReservedOutputsAsNotSpendable_RejectsAlreadyClaimedOutput(t *testing.T) {
	// given:
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB)
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "claim-contention-user", "test-storage")
	require.NoError(t, err)

	// owner: the transaction that originally created output O.
	require.NoError(t, repos.CreateTransaction(ctx, &storageentity.NewTx{
		UserID:    user.ID,
		Status:    wdk.TxStatusCompleted,
		Reference: "claim-contention-owner-tx",
		Version:   1,
	}))
	ownerTx, err := repos.FindTransactionByReference(ctx, user.ID, "claim-contention-owner-tx")
	require.NoError(t, err)
	require.NotNil(t, ownerTx)

	// spender: the transaction that already claimed (spent_by) output O.
	require.NoError(t, repos.CreateTransaction(ctx, &storageentity.NewTx{
		UserID:    user.ID,
		Status:    wdk.TxStatusCompleted,
		Reference: "claim-contention-spender-tx",
		Version:   1,
	}))
	spenderTx, err := repos.FindTransactionByReference(ctx, user.ID, "claim-contention-spender-tx")
	require.NoError(t, err)
	require.NotNil(t, spenderTx)

	// O: a non-change output already claimed by the spender transaction. This
	// mirrors the shape seeded by funder/testabilities/fixture_utxo.go's Stored(),
	// minus the bsv_user_utxos row (KnownOutputIDs outputs never get one).
	claimedOutput := models.Output{
		UserID:        user.ID,
		TransactionID: ownerTx.ID,
		SpentBy:       &spenderTx.ID,
		Vout:          0,
		Satoshis:      1000,
		Spendable:     false,
		Change:        false,
		ProvidedBy:    string(wdk.ProvidedByYou),
		Purpose:       "",
		Type:          "P2PKH",
	}
	require.NoError(t, db.DB.Create(&claimedOutput).Error)

	// when: tx B tries to claim the already-spent output O via SpentOutputIDs.
	newTxB := &storageentity.NewTx{
		UserID:         user.ID,
		Status:         wdk.TxStatusUnsigned,
		Reference:      "claim-contention-tx-b",
		Version:        1,
		SpentOutputIDs: []uint{claimedOutput.ID},
	}

	createErr := db.DB.Transaction(func(tx *gorm.DB) error {
		return repos.CreateTransactionInTx(ctx, tx, newTxB)
	})

	// then:
	require.Error(t, createErr)
	require.ErrorIs(t, createErr, repo.ErrUTXOContention)

	var count int64
	require.NoError(t, db.DB.Model(&models.Transaction{}).
		Where("reference = ?", newTxB.Reference).
		Count(&count).Error)
	require.Zerof(t, count, "tx B's transaction row should have been rolled back, found %d", count)
}
