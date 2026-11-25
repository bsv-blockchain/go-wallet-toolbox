package repo_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
)

const hotQueryUserID = 1

// seedHotQueryOutputs inserts `others` filler transactions (one spendable change output each,
// spent_by NULL, mimicking a wallet that has been running for a while) and one target
// transaction with two spendable change outputs. It returns the target's id and tx_id.
func seedHotQueryOutputs(tb testing.TB, db *gorm.DB, others int) (uint, string) {
	tb.Helper()

	basket := "default"
	targetTxID := fmt.Sprintf("%064x", others+1)

	txs := make([]*models.Transaction, 0, others+1)
	for i := 0; i <= others; i++ {
		txID := fmt.Sprintf("%064x", i+1)
		txs = append(txs, &models.Transaction{
			UserID:    hotQueryUserID,
			Status:    "completed",
			Reference: fmt.Sprintf("ref-%d", i),
			TxID:      &txID,
		})
	}
	require.NoError(tb, db.CreateInBatches(txs, 500).Error)

	outs := make([]*models.Output, 0, others+2)
	for _, tx := range txs {
		outs = append(outs, &models.Output{
			UserID: hotQueryUserID, TransactionID: tx.ID, Vout: 0, Satoshis: 1000,
			Spendable: true, Change: true, BasketName: &basket, Type: "P2PKH",
		})
	}
	target := txs[len(txs)-1]
	outs = append(outs, &models.Output{
		UserID: hotQueryUserID, TransactionID: target.ID, Vout: 1, Satoshis: 2000,
		Spendable: true, Change: true, BasketName: &basket, Type: "P2PKH",
	})
	require.NoError(tb, db.CreateInBatches(outs, 500).Error)

	return target.ID, targetTxID
}

// BenchmarkHotOutputQueries measures the per-call cost of the two per-transaction output lookups
// that run on every CreateAction: the change-output find in SpendTransaction and the
// spendable-change find behind CreateUTXOForSpendableOutputsByTxID. Each sub-benchmark seeds a
// table of n spendable change outputs; the cost per call should stay flat as n grows (it grew
// linearly when SQLite scanned the user_id / spendable index).
func BenchmarkHotOutputQueries(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 50_000} {
		b.Run(fmt.Sprintf("outputs_%d", n), func(b *testing.B) {
			db, cleanup := dbfixtures.TestDatabase(b)
			defer cleanup()

			transactionID, txID := seedHotQueryOutputs(b, db.DB, n)

			findChange := repo.NewUnspentChangeOutputsFinderForTest(db.DB)
			findSpendable := repo.NewSpendableChangeOutputsFinderForTest(db.DB)

			b.Run("SpendTransactionChangeFind", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					found, err := findChange(hotQueryUserID, transactionID)
					require.NoError(b, err)
					require.Len(b, found, 2)
				}
			})

			b.Run("SpendableChangeByTxID", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					count, err := findSpendable(b.Context(), txID)
					require.NoError(b, err)
					require.Equal(b, 2, count)
				}
			})
		})
	}
}
