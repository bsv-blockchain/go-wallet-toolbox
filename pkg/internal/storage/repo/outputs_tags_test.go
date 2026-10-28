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
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TestOutputs_ManyDistinctTags is the tag twin of
// TestFindTransactions_ManyDistinctLabels: Tag's primary key is (name, user_id),
// so GORM's Preload of the many2many Tags selected them with a row-value
// `(name, user_id) IN (...)` list that Postgres fails to evaluate past a few
// thousand distinct tags ("stack depth limit exceeded"; run with
// TEST_DB_MODE=postgres to see it). A wallet that tags every token output with
// its own ID gets there with one page of ListOutputs.
//
// given: one transaction with 9,000 outputs, each with its own tag plus a
// shared one, a soft-deleted tag link and a link to a soft-deleted tag.
// when:  ListAndCountOutputs with tags, and FindInputsAndOutputsWithBaskets.
// then:  every output comes back with exactly its live tags.
func TestOutputs_ManyDistinctTags(t *testing.T) {
	const outputCount = 9000

	// given:
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	repos := repo.NewSQLRepositories(db.DB, dbretry.NoRetry())
	ctx := t.Context()

	user, err := repos.CreateUser(ctx, "many-tags-user", "test-storage")
	require.NoError(t, err)

	require.NoError(t, repos.CreateTransaction(ctx, &entity.NewTx{
		UserID: user.ID, Status: wdk.TxStatusCompleted, Reference: "many-tags-tx", Version: 1,
	}))
	tx, err := repos.FindTransactionByReference(ctx, user.ID, "many-tags-tx")
	require.NoError(t, err)

	outputs := make([]*models.Output, outputCount)
	for i := range outputs {
		outputs[i] = &models.Output{UserID: user.ID, TransactionID: tx.ID, Vout: uint32(i), Satoshis: 1, Spendable: true} //nolint:gosec // i < outputCount
	}
	require.NoError(t, db.DB.CreateInBatches(outputs, 500).Error)

	tags := []*models.Tag{{Name: "token", UserID: user.ID}, {Name: "removed", UserID: user.ID}}
	links := make([]*models.OutputTag, 0, 2*outputCount+2)
	for i, out := range outputs {
		name := fmt.Sprintf("pushdrop-%d", i)
		tags = append(tags, &models.Tag{Name: name, UserID: user.ID})
		links = append(links,
			&models.OutputTag{OutputID: out.ID, TagName: name, TagUserID: user.ID},
			&models.OutputTag{OutputID: out.ID, TagName: "token", TagUserID: user.ID},
		)
	}
	deletedAt := gorm.DeletedAt{Time: time.Now(), Valid: true}
	// soft-deleted tag "removed", still linked to output 1
	tags[1].DeletedAt = deletedAt
	links = append(links,
		&models.OutputTag{OutputID: outputs[1].ID, TagName: "removed", TagUserID: user.ID},
		// soft-deleted link from output 0 to a live tag
		&models.OutputTag{OutputID: outputs[0].ID, TagName: "pushdrop-2", TagUserID: user.ID, DeletedAt: deletedAt},
	)
	require.NoError(t, db.DB.CreateInBatches(tags, 500).Error)
	require.NoError(t, db.DB.CreateInBatches(links, 500).Error)

	requireTags := func(t *testing.T, got []*pkgentity.Output) {
		t.Helper()
		require.Len(t, got, outputCount)
		byVout := make(map[uint32][]string, len(got))
		for _, out := range got {
			byVout[out.Vout] = out.Tags
		}
		require.ElementsMatch(t, []string{"pushdrop-0", "token"}, byVout[0], "soft-deleted link must be skipped")
		require.ElementsMatch(t, []string{"pushdrop-1", "token"}, byVout[1], "soft-deleted tag must be skipped")
		require.ElementsMatch(t, []string{"pushdrop-8999", "token"}, byVout[outputCount-1])
	}

	t.Run("ListAndCountOutputs", func(t *testing.T) {
		// when:
		listed, total, err := repos.ListAndCountOutputs(ctx, entity.ListOutputsFilter{
			UserID: user.ID, Limit: 10000, IncludeTags: true,
		})

		// then:
		require.NoError(t, err)
		require.EqualValues(t, outputCount, total)
		requireTags(t, listed)
	})

	t.Run("FindInputsAndOutputsWithBaskets", func(t *testing.T) {
		// when:
		_, byTx, err := repos.FindInputsAndOutputsWithBaskets(ctx, []uint{tx.ID}, false)

		// then:
		require.NoError(t, err)
		requireTags(t, byTx[tx.ID])
	})
}
