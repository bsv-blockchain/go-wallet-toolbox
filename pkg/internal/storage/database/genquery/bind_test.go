package genquery_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
)

func TestQuery_TransactionBinding(t *testing.T) {
	ctx := context.Background()

	// given: a database with two connections, so reads outside the transaction do not wait for it
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "bind.sqlite")+"?_journal_mode=WAL"), &gorm.Config{
		Logger: logger.Discard,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.KnownTx{}))

	base := genquery.Use(db)

	t.Run("runs on the transaction's connection and keeps the model", func(t *testing.T) {
		// given:
		tx := db.WithContext(ctx).Begin()
		require.NoError(t, tx.Error)
		bound := base.BindTx(tx)

		// when:
		err := bound.KnownTx.WithContext(ctx).Create(&models.KnownTx{TxID: "a1", Attempts: 1})
		require.NoError(t, err)

		// then: the transaction sees its own write, the connection outside it does not
		insideCount, err := bound.KnownTx.WithContext(ctx).Where(bound.KnownTx.TxID.Eq("a1")).Count()
		require.NoError(t, err)
		require.EqualValues(t, 1, insideCount)

		outsideCount, err := base.KnownTx.WithContext(ctx).Where(base.KnownTx.TxID.Eq("a1")).Count()
		require.NoError(t, err)
		require.Zero(t, outsideCount)

		// and: an update through the bound copy targets the model's table inside the transaction
		_, err = bound.KnownTx.WithContext(ctx).Where(bound.KnownTx.TxID.Eq("a1")).Update(bound.KnownTx.Attempts, 2)
		require.NoError(t, err)

		// when: the transaction rolls back
		require.NoError(t, tx.Rollback().Error)

		// then: nothing was written
		afterRollback, err := base.KnownTx.WithContext(ctx).Count()
		require.NoError(t, err)
		require.Zero(t, afterRollback)
	})

	t.Run("commits with the transaction and leaves the base query untouched", func(t *testing.T) {
		// given:
		tx := db.WithContext(ctx).Begin()
		require.NoError(t, tx.Error)
		bound := base.BindTx(tx)

		// when:
		require.NoError(t, bound.KnownTx.WithContext(ctx).Create(&models.KnownTx{TxID: "b1"}))
		require.NoError(t, tx.Commit().Error)

		// then: the base query, still on the pool, reads the committed row
		got, err := base.KnownTx.WithContext(ctx).Where(base.KnownTx.TxID.Eq("b1")).First()
		require.NoError(t, err)
		require.Equal(t, "b1", got.TxID)
	})
	t.Run("RunInTransaction on the pool runs fc on a copy bound to a new transaction", func(t *testing.T) {
		// when:
		var inner *genquery.Query
		err := base.RunInTransaction(func(tx *genquery.Query) error {
			inner = tx
			return tx.KnownTx.WithContext(ctx).Create(&models.KnownTx{TxID: "c1"})
		})

		// then:
		require.NoError(t, err)
		require.NotSame(t, base, inner)
		count, err := base.KnownTx.WithContext(ctx).Where(base.KnownTx.TxID.Eq("c1")).Count()
		require.NoError(t, err)
		require.EqualValues(t, 1, count)
	})

	t.Run("RunInTransaction nested in a transaction reuses the bound query in a savepoint", func(t *testing.T) {
		// given:
		tx := db.WithContext(ctx).Begin()
		require.NoError(t, tx.Error)
		bound := base.BindTx(tx)
		require.NoError(t, bound.KnownTx.WithContext(ctx).Create(&models.KnownTx{TxID: "d1"}))

		// when: a nested transaction writes and fails
		var inner *genquery.Query
		failed := errors.New("nested failure")
		err := bound.RunInTransaction(func(nested *genquery.Query) error {
			inner = nested
			require.NoError(t, nested.KnownTx.WithContext(ctx).Create(&models.KnownTx{TxID: "d2"}))
			return failed
		})

		// then: it ran on the bound query, and only its own write was rolled back
		require.ErrorIs(t, err, failed)
		require.Same(t, bound, inner)

		got, err := bound.KnownTx.WithContext(ctx).Where(bound.KnownTx.TxID.In("d1", "d2")).Find()
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "d1", got[0].TxID)

		// when: a nested transaction succeeds, and the outer one commits
		require.NoError(t, bound.RunInTransaction(func(nested *genquery.Query) error {
			return nested.KnownTx.WithContext(ctx).Create(&models.KnownTx{TxID: "d3"})
		}))
		require.NoError(t, tx.Commit().Error)

		// then:
		committed, err := base.KnownTx.WithContext(ctx).Where(base.KnownTx.TxID.In("d1", "d2", "d3")).Count()
		require.NoError(t, err)
		require.EqualValues(t, 2, committed)
	})
}
