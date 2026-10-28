package repo_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	pkgentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TestFindTransactions_ManyDistinctLabels covers loading the labels of a user
// with thousands of transactions. Label's primary key is (name, user_id), so
// GORM's Preload of the many2many Labels selected them with a row-value
// `(name, user_id) IN ((..),(..),...)` list, one tuple per distinct label;
// Postgres fails that with "stack depth limit exceeded" somewhere between 5,000
// and 8,000 tuples (run with TEST_DB_MODE=postgres to see it).
//
// given: 9,000 transactions, each with its own label plus a shared one, a
// soft-deleted label link and a link to a soft-deleted label.
// when:  FindTransactions for the user.
// then:  every transaction comes back with exactly its live labels.
func TestFindTransactions_ManyDistinctLabels(t *testing.T) {
	const txCount = 9000

	// given:
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "many-labels-user", "test-storage")
	require.NoError(t, err)

	txs := make([]*models.Transaction, txCount)
	for i := range txs {
		txs[i] = &models.Transaction{UserID: user.ID, Status: wdk.TxStatusCompleted, Reference: fmt.Sprintf("many-labels-%d", i), Version: 1}
	}
	require.NoError(t, db.DB.CreateInBatches(txs, 500).Error)

	labels := make([]*models.Label, 0, txCount+2)
	labels = append(labels, &models.Label{Name: "kind=token", UserID: user.ID}, &models.Label{Name: "removed", UserID: user.ID})
	links := make([]*models.TransactionLabel, 0, 2*txCount+2)
	for i, tx := range txs {
		name := fmt.Sprintf("id=%d", i)
		labels = append(labels, &models.Label{Name: name, UserID: user.ID})
		links = append(links,
			&models.TransactionLabel{TransactionID: tx.ID, LabelName: name, LabelUserID: user.ID},
			&models.TransactionLabel{TransactionID: tx.ID, LabelName: "kind=token", LabelUserID: user.ID},
		)
	}
	deletedAt := gorm.DeletedAt{Time: time.Now(), Valid: true}
	// soft-deleted label "removed", still linked to tx 1
	labels[1].DeletedAt = deletedAt
	links = append(links,
		&models.TransactionLabel{TransactionID: txs[1].ID, LabelName: "removed", LabelUserID: user.ID},
		// soft-deleted link from tx 0 to a live label
		&models.TransactionLabel{TransactionID: txs[0].ID, LabelName: "id=2", LabelUserID: user.ID, DeletedAt: deletedAt},
	)
	require.NoError(t, db.DB.CreateInBatches(labels, 500).Error)
	require.NoError(t, db.DB.CreateInBatches(links, 500).Error)

	// when:
	found, err := repos.FindTransactions(ctx, &pkgentity.TransactionReadSpecification{
		UserID: &pkgentity.Comparable[int]{Value: user.ID, Cmp: pkgentity.Equal},
	})

	// then:
	require.NoError(t, err)
	require.Len(t, found, txCount)
	byRef := make(map[string][]string, len(found))
	for _, tx := range found {
		byRef[tx.Reference] = tx.Labels
	}
	require.ElementsMatch(t, []string{"id=0", "kind=token"}, byRef["many-labels-0"], "soft-deleted link must be skipped")
	require.ElementsMatch(t, []string{"id=1", "kind=token"}, byRef["many-labels-1"], "soft-deleted label must be skipped")
	require.ElementsMatch(t, []string{"id=8999", "kind=token"}, byRef["many-labels-8999"])
}
