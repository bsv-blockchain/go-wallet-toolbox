package repo_test

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	storageentity "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type basketFixture struct {
	t       *testing.T
	db      *gorm.DB
	repos   *repo.Repositories
	inserts atomic.Int64
	seq     int
}

func newBasketFixture(t *testing.T) *basketFixture {
	t.Helper()
	return newBasketFixtureWith(t)
}

func newBasketFixtureWith(t *testing.T, modifiers ...dbfixtures.DBConfigModifier) *basketFixture {
	t.Helper()
	db, cleanup := dbfixtures.TestDatabase(t, modifiers...)
	t.Cleanup(cleanup)

	f := &basketFixture{t: t, db: db.DB, repos: repo.NewSQLRepositories(db.DB, dbretry.NoRetry())}
	require.NoError(t, db.DB.Callback().Create().Before("gorm:create").Register("count_basket_inserts", func(d *gorm.DB) {
		if d.Statement.Table == "bsv_output_baskets" {
			f.inserts.Add(1)
		}
	}))

	return f
}

func (f *basketFixture) newTx(userID int, basket string) *storageentity.NewTx {
	f.seq++
	name := basket
	return &storageentity.NewTx{
		UserID:    userID,
		Status:    wdk.TxStatusUnsigned,
		Reference: "basket-cache-ref-" + string(rune('a'+f.seq%26)) + string(rune('a'+f.seq/26)),
		Version:   1,
		Outputs: []*storageentity.NewOutput{{
			Satoshis:   satoshi.MustFrom(1000),
			BasketName: &name,
			Spendable:  true,
			ProvidedBy: wdk.ProvidedByYou,
			Type:       wdk.OutputTypeP2PKH,
			Purpose:    "test",
			Vout:       0,
		}},
	}
}

func (f *basketFixture) basketExists(userID int, name string) bool {
	var n int64
	require.NoError(f.t, f.db.Unscoped().Model(&models.OutputBasket{}).Where("user_id = ? AND name = ?", userID, name).Count(&n).Error)
	return n == 1
}

func TestBasketCache_SkipsInsertAfterBasketIsKnownCommitted(t *testing.T) {
	f := newBasketFixture(t)
	ctx := t.Context()
	user, err := f.repos.CreateUser(ctx, "basket-cache-user", "s", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	base := f.inserts.Load() // creating the user inserts its default baskets
	counts := make([]int64, 0, 5)
	for range 5 {
		require.NoError(t, f.repos.CreateTransaction(ctx, f.newTx(user.ID, "custom")))
		counts = append(counts, f.inserts.Load()-base)
	}

	// 1st call inserts the row, 2nd finds it (0 rows) and remembers it, the rest skip the statement.
	assert.Equal(t, []int64{1, 2, 2, 2, 2}, counts)
	assert.True(t, f.basketExists(user.ID, "custom"))
}

func TestBasketCache_ForgetBasketRecreatesDeletedBasket(t *testing.T) {
	f := newBasketFixture(t)
	ctx := t.Context()
	user, err := f.repos.CreateUser(ctx, "basket-cache-user", "s", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	for range 3 {
		require.NoError(t, f.repos.CreateTransaction(ctx, f.newTx(user.ID, "custom")))
	}
	before := f.inserts.Load()

	// the basket row is hard-deleted; the explicit invalidation makes the next call re-create it
	require.NoError(t, f.db.Unscoped().Where("user_id = ? AND name = ?", user.ID, "custom").Delete(&models.OutputBasket{}).Error)
	f.repos.Transactions.ForgetBasket(user.ID, "custom")

	require.NoError(t, f.repos.CreateTransaction(ctx, f.newTx(user.ID, "custom")))
	assert.Equal(t, before+1, f.inserts.Load())
	assert.True(t, f.basketExists(user.ID, "custom"))
}

func TestBasketCache_RolledBackInsertIsNotRemembered(t *testing.T) {
	f := newBasketFixture(t)
	ctx := t.Context()
	user, err := f.repos.CreateUser(ctx, "basket-cache-user", "s", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	// Two creations in ONE outer transaction: the first inserts the basket, the second sees its
	// own uncommitted row (0 rows). Neither may be remembered, because the transaction rolls back.
	errRollback := assert.AnError
	err = f.db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, f.repos.Transactions.CreateTransactionInTx(ctx, tx, f.newTx(user.ID, "custom")))
		require.NoError(t, f.repos.Transactions.CreateTransactionInTx(ctx, tx, f.newTx(user.ID, "custom")))
		return errRollback
	})
	require.ErrorIs(t, err, errRollback)
	require.False(t, f.basketExists(user.ID, "custom"), "rollback removed the basket")

	// the next call must insert the basket again instead of trusting a phantom entry
	require.NoError(t, f.repos.CreateTransaction(ctx, f.newTx(user.ID, "custom")))
	assert.True(t, f.basketExists(user.ID, "custom"))
}

func TestBasketCache_IsPerUser(t *testing.T) {
	f := newBasketFixture(t)
	ctx := t.Context()
	first, err := f.repos.CreateUser(ctx, "basket-cache-user-1", "s", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)
	second, err := f.repos.CreateUser(ctx, "basket-cache-user-2", "s", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	for range 3 {
		require.NoError(t, f.repos.CreateTransaction(ctx, f.newTx(first.ID, "custom")))
	}
	require.True(t, f.basketExists(first.ID, "custom"))
	require.False(t, f.basketExists(second.ID, "custom"))

	// a new user does not inherit the other user's cached basket
	require.NoError(t, f.repos.CreateTransaction(ctx, f.newTx(second.ID, "custom")))
	assert.True(t, f.basketExists(second.ID, "custom"))
}

func TestBasketCache_ConcurrentActionsAreSafe(t *testing.T) {
	f := newBasketFixture(t)
	ctx := t.Context()
	user, err := f.repos.CreateUser(ctx, "basket-cache-user", "s", wdk.DefaultBasketConfiguration())
	require.NoError(t, err)

	txs := make([]*storageentity.NewTx, 8)
	for i := range txs {
		txs[i] = f.newTx(user.ID, "custom")
	}
	errs := make(chan error, len(txs))
	for _, nt := range txs {
		go func() { errs <- f.repos.CreateTransaction(ctx, nt) }()
	}
	for range txs {
		require.NoError(t, <-errs)
	}
	assert.True(t, f.basketExists(user.ID, "custom"))
}
