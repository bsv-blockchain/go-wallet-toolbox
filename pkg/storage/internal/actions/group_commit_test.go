package actions

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/groupcommit"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// TestRunFunding_GroupedContentionIsRetriedOutsideTheSharedTransaction drives the funding retry
// loop through a real group committer: a contention attempt and a stale-index attempt each roll
// back only their own writes, the retry (with its backoff) happens outside any transaction, and
// the third attempt commits.
func TestRunFunding_GroupedContentionIsRetriedOutsideTheSharedTransaction(t *testing.T) {
	withShrunkFundingBackoff(t)

	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	logger := logging.NewTestLogger(t)
	committer := groupcommit.New(db.DB, logger, defs.GroupCommit{Enabled: true}, NewGroupScope)
	defer func() { require.NoError(t, committer.Close(context.Background())) }()

	c := &create{db: db.DB, logger: logger, group: committer}

	calls := 0
	fundingClosure := func(ctx context.Context, dbTx *gorm.DB) error {
		calls++
		if err := dbTx.WithContext(ctx).Create(&models.KeyValue{Key: fmt.Sprintf("attempt-%d", calls), Value: []byte{1}}).Error; err != nil {
			return err
		}
		switch calls {
		case 1:
			return fmt.Errorf("reserving selected utxos: %w", wdk.ErrUTXOContention)
		case 2:
			return &repo.StaleUTXOIndexError{OutputIDs: []uint{7}}
		default:
			return nil
		}
	}

	err := retryOnContention(t.Context(), logger, randomizer.NewTestRandomizer(), func() error {
		return c.runFunding(t.Context(), fundingClosure)
	})

	require.NoError(t, err)
	assert.Equal(t, 3, calls)

	var keys []string
	require.NoError(t, db.DB.Model(&models.KeyValue{}).Where("key LIKE ?", "attempt-%").Order("key").Pluck("key", &keys).Error)
	assert.Equal(t, []string{"attempt-3"}, keys, "the failed attempts' writes were rolled back to their savepoints")

	batches, units := committer.Stats()
	assert.Equal(t, uint64(3), batches, "every attempt is a unit of its own; the backoff runs between batches")
	assert.Equal(t, uint64(3), units)
}

// TestRunFunding_WithoutGroupCommitUsesItsOwnTransaction pins the ungrouped path.
func TestRunFunding_WithoutGroupCommitUsesItsOwnTransaction(t *testing.T) {
	db, cleanup := dbfixtures.TestDatabase(t)
	defer cleanup()

	c := &create{db: db.DB, logger: logging.NewTestLogger(t)}

	var inTx bool
	err := c.runFunding(t.Context(), func(_ context.Context, dbTx *gorm.DB) error {
		_, inTx = dbTx.Statement.ConnPool.(gorm.TxCommitter)
		return nil
	})

	require.NoError(t, err)
	assert.True(t, inTx)
}
