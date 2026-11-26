package groupcommit_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/groupcommit"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testmode"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

type gcRow struct {
	ID     uint   `gorm:"primaryKey"`
	Caller string `gorm:"uniqueIndex"`
}

func (gcRow) TableName() string { return "gc_rows" }

var errUnit = errors.New("unit failed")

// testDBs runs fn against the fixture database (in-memory SQLite on ONE connection by default, or
// Postgres under TEST_DB_MODE=postgres) and against a file SQLite with _txlock=immediate and a
// pool of 4, which is how write-heavy deployments run it.
func testDBs(t *testing.T, fn func(t *testing.T, db *gorm.DB)) {
	t.Run("fixture", func(t *testing.T) {
		db, cleanup := dbfixtures.TestDatabase(t)
		t.Cleanup(cleanup)
		require.NoError(t, db.DB.AutoMigrate(&gcRow{}))
		fn(t, db.DB)
	})

	t.Run("sqlite_file_immediate", func(t *testing.T) {
		cfg := defs.DefaultDBConfig()
		cfg.SQLite.ConnectionString = "file:" + filepath.Join(t.TempDir(), "gc.sqlite") + "?_txlock=immediate"
		cfg.MaxOpenConnections = 4
		cfg.MaxIdleConnections = 4
		db, err := database.NewDatabase(cfg, logging.NewTestLogger(t))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		require.NoError(t, db.DB.AutoMigrate(&gcRow{}))
		fn(t, db.DB)
	})
}

// requireNoCommitterGoroutines fails when a goroutine of a Committer is still running (the repo
// does not use goleak; this is the same check scoped to this package's goroutines).
func requireNoCommitterGoroutines(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		buf := make([]byte, 1<<20)
		buf = buf[:runtime.Stack(buf, true)]
		return !strings.Contains(string(buf), "groupcommit.(*Committer)")
	}, 5*time.Second, 10*time.Millisecond, "committer goroutines must exit on Close")
}

func inMemoryFixture(t *testing.T) bool {
	return testmode.GetMode() == nil && strings.HasSuffix(t.Name(), "/fixture")
}

// txScope makes the shared transaction itself the batch scope.
func txScope(tx *gorm.DB) *gorm.DB { return tx }

func newCommitter(t *testing.T, db *gorm.DB, cfg defs.GroupCommit) *groupcommit.Committer[*gorm.DB] {
	t.Helper()
	c := groupcommit.New(db, logging.NewTestLogger(t), cfg, txScope)
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func insert(caller string) groupcommit.Work[*gorm.DB] {
	return func(ctx context.Context, tx *gorm.DB) error {
		return tx.WithContext(ctx).Create(&gcRow{Caller: caller}).Error
	}
}

func soloInsert(db *gorm.DB, caller string) groupcommit.Solo {
	return func(ctx context.Context) error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return insert(caller)(ctx, tx)
		})
	}
}

func callers(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var rows []gcRow
	require.NoError(t, db.Order("caller").Find(&rows).Error)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Caller)
	}
	return out
}

// blocker is a unit of work that holds its batch open until released, so that the next units
// queue up behind it and land in one batch together.
type blocker struct {
	started chan struct{}
	release chan struct{}
}

func newBlocker() *blocker {
	return &blocker{started: make(chan struct{}), release: make(chan struct{})}
}

func (b *blocker) work(caller string) groupcommit.Work[*gorm.DB] {
	return func(ctx context.Context, tx *gorm.DB) error {
		close(b.started)
		<-b.release
		return insert(caller)(ctx, tx)
	}
}

// queueBehind starts the blocker unit and returns once it holds the worker.
func queueBehind(t *testing.T, c *groupcommit.Committer[*gorm.DB], db *gorm.DB, b *blocker, wg *sync.WaitGroup) {
	t.Helper()
	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.NoError(t, c.Do(context.Background(), b.work("blocker"), soloInsert(db, "blocker")))
	}()
	<-b.started
}

func TestCommitter_MixedSuccessAndFailureInOneBatch(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := newCommitter(t, db, defs.GroupCommit{Enabled: true, MaxBatch: 16})

		var wg sync.WaitGroup
		b := newBlocker()
		queueBehind(t, c, db, b, &wg)

		// A Go-level failure after a write, a SQL-level failure (unique violation, which aborts the
		// subtransaction on Postgres), and successes, all queued into the same next batch.
		const n = 8
		errs := make([]error, n)
		for i := range n {
			caller := fmt.Sprintf("c%d", i)
			var work groupcommit.Work[*gorm.DB]
			switch i % 4 {
			case 1:
				work = func(ctx context.Context, tx *gorm.DB) error {
					if err := insert(caller)(ctx, tx); err != nil {
						return err
					}
					return errUnit
				}
			case 3:
				work = insert("blocker") // duplicate of the blocker's committed row
			default:
				work = insert(caller)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = c.Do(context.Background(), work, func(context.Context) error {
					return errors.New("solo must not run")
				})
			}()
		}

		waitQueued(t, c, n)
		close(b.release)
		wg.Wait()

		for i, err := range errs {
			switch i % 4 {
			case 1:
				require.ErrorIs(t, err, errUnit, "unit %d", i)
			case 3:
				require.Error(t, err, "unit %d: duplicate insert must fail", i)
				require.NotErrorIs(t, err, errUnit)
			default:
				require.NoError(t, err, "unit %d", i)
			}
		}
		assert.Equal(t, []string{"blocker", "c0", "c2", "c4", "c6"}, callers(t, db))

		batches, units := c.Stats()
		assert.Equal(t, uint64(2), batches, "the blocker's batch, then all eight units together")
		assert.Equal(t, uint64(1+n), units)
	})
}

// waitQueued waits until n units sit in the committer's queue behind a running batch.
func waitQueued(t *testing.T, c *groupcommit.Committer[*gorm.DB], n int) {
	t.Helper()
	require.Eventually(t, func() bool { return c.Queued() == n }, 5*time.Second, time.Millisecond)
}

func TestCommitter_SplitsByMaxBatch(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := newCommitter(t, db, defs.GroupCommit{Enabled: true, MaxBatch: 3})

		var wg sync.WaitGroup
		b := newBlocker()
		queueBehind(t, c, db, b, &wg)

		for i := range 7 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				caller := fmt.Sprintf("c%d", i)
				assert.NoError(t, c.Do(context.Background(), insert(caller), soloInsert(db, caller)))
			}()
		}
		waitQueued(t, c, 7)
		close(b.release)
		wg.Wait()

		batches, units := c.Stats()
		assert.Equal(t, uint64(1+3), batches, "blocker, then 3+3+1")
		assert.Equal(t, uint64(8), units)
		assert.Len(t, callers(t, db), 8)
	})
}

func TestCommitter_EngineConflictRunsSolo(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := newCommitter(t, db, defs.GroupCommit{Enabled: true})

		var wg sync.WaitGroup
		b := newBlocker()
		queueBehind(t, c, db, b, &wg)

		var soloRuns int
		var conflictErr, mateErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			conflictErr = c.Do(context.Background(),
				func(ctx context.Context, tx *gorm.DB) error {
					if err := insert("conflict-grouped")(ctx, tx); err != nil {
						return err
					}
					return fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "40P01", Message: "deadlock detected"})
				},
				func(ctx context.Context) error {
					soloRuns++
					return soloInsert(db, "conflict-solo")(ctx)
				})
		}()
		go func() {
			defer wg.Done()
			mateErr = c.Do(context.Background(), insert("mate"), soloInsert(db, "mate"))
		}()
		waitQueued(t, c, 2)
		close(b.release)
		wg.Wait()

		require.NoError(t, conflictErr)
		require.NoError(t, mateErr)
		assert.Equal(t, 1, soloRuns)
		assert.Equal(t, []string{"blocker", "conflict-solo", "mate"}, callers(t, db),
			"the conflicted unit's grouped write is rolled back and its solo path committed")
	})
}

func TestCommitter_CancelWhilePendingRunsSoloWithCanceledContext(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := newCommitter(t, db, defs.GroupCommit{Enabled: true})

		var wg sync.WaitGroup
		b := newBlocker()
		queueBehind(t, c, db, b, &wg)

		ctx, cancel := context.WithCancel(context.Background())
		workRan := make(chan struct{}, 1)
		result := make(chan error, 1)
		go func() {
			result <- c.Do(ctx,
				func(ctx context.Context, tx *gorm.DB) error {
					workRan <- struct{}{}
					return insert("canceled")(ctx, tx)
				},
				soloInsert(db, "canceled"))
		}()
		waitQueued(t, c, 1)
		cancel()

		// The caller returns while the batch ahead of it is still open.
		select {
		case err := <-result:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(5 * time.Second):
			t.Fatal("canceled caller did not return")
		}

		close(b.release)
		wg.Wait()

		require.Never(t, func() bool { return len(workRan) > 0 }, 50*time.Millisecond, 5*time.Millisecond)
		assert.Equal(t, []string{"blocker"}, callers(t, db))
	})
}

func TestCommitter_CancelWhileRunningDoesNotBreakTheBatch(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := newCommitter(t, db, defs.GroupCommit{Enabled: true})

		var wg sync.WaitGroup
		b := newBlocker()
		queueBehind(t, c, db, b, &wg)

		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{})
		canceled := make(chan struct{})
		var canceledErr, mateErr error

		wg.Add(2)
		go func() {
			defer wg.Done()
			canceledErr = c.Do(ctx,
				func(ctx context.Context, tx *gorm.DB) error {
					if err := insert("canceled-1")(ctx, tx); err != nil {
						return err
					}
					close(started)
					<-canceled
					// The caller's context is canceled by now; the unit's statements must not see it.
					return insert("canceled-2")(ctx, tx)
				},
				func(context.Context) error { return errors.New("solo must not run") })
		}()
		waitQueued(t, c, 1)
		go func() {
			defer wg.Done()
			mateErr = c.Do(context.Background(), insert("mate"), soloInsert(db, "mate"))
		}()
		waitQueued(t, c, 2)

		close(b.release)
		<-started
		cancel()
		close(canceled)
		wg.Wait()

		require.NoError(t, canceledErr, "a caller canceled after its unit started gets the committed outcome")
		require.NoError(t, mateErr)
		assert.Equal(t, []string{"blocker", "canceled-1", "canceled-2", "mate"}, callers(t, db))
	})
}

func TestCommitter_PanicIsRaisedInTheCallerAndMatesCommit(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := newCommitter(t, db, defs.GroupCommit{Enabled: true})

		var wg sync.WaitGroup
		b := newBlocker()
		queueBehind(t, c, db, b, &wg)

		var recovered any
		var mateErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			defer func() { recovered = recover() }()
			_ = c.Do(context.Background(), func(ctx context.Context, tx *gorm.DB) error {
				if err := insert("panicker")(ctx, tx); err != nil {
					return err
				}
				panic("boom")
			}, soloInsert(db, "panicker"))
		}()
		waitQueued(t, c, 1)
		go func() {
			defer wg.Done()
			mateErr = c.Do(context.Background(), insert("mate"), soloInsert(db, "mate"))
		}()
		waitQueued(t, c, 2)
		close(b.release)
		wg.Wait()

		assert.Equal(t, "boom", recovered)
		require.NoError(t, mateErr)
		assert.Equal(t, []string{"blocker", "mate"}, callers(t, db))
	})
}

func TestCommitter_CloseDrainsQueuedWorkAndLeaksNoGoroutines(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := groupcommit.New(db, logging.NewTestLogger(t), defs.GroupCommit{Enabled: true, MaxBatch: 2}, txScope)

		var wg sync.WaitGroup
		b := newBlocker()
		queueBehind(t, c, db, b, &wg)

		for i := range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				caller := fmt.Sprintf("c%d", i)
				assert.NoError(t, c.Do(context.Background(), insert(caller), func(context.Context) error {
					return errors.New("queued work must be drained by the committer, not run solo")
				}))
			}()
		}
		waitQueued(t, c, 5)

		closed := make(chan error, 1)
		go func() { closed <- c.Close(context.Background()) }()

		close(b.release)
		require.NoError(t, <-closed)
		wg.Wait()

		// Work submitted after Close runs on its ungrouped path.
		soloRan := false
		require.NoError(t, c.Do(context.Background(), insert("late-grouped"), func(ctx context.Context) error {
			soloRan = true
			return soloInsert(db, "late")(ctx)
		}))
		assert.True(t, soloRan)

		assert.Equal(t, []string{"blocker", "c0", "c1", "c2", "c3", "c4", "late"}, callers(t, db))
		batches, _ := c.Stats()
		assert.Equal(t, uint64(1+3), batches, "blocker, then the queue drained as 2+2+1")
		requireNoCommitterGoroutines(t)
	})
}

func TestCommitter_MultipleWorkers(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := newCommitter(t, db, defs.GroupCommit{Enabled: true, MaxBatch: 3, Workers: 3})

		const n = 30
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				caller := fmt.Sprintf("c%02d", i)
				assert.NoError(t, c.Do(context.Background(), insert(caller), soloInsert(db, caller)))
			}()
		}
		wg.Wait()

		assert.Len(t, callers(t, db), n)
		_, units := c.Stats()
		assert.Equal(t, uint64(n), units)
	})
}

func TestCommitter_CloseDeadlineCancelsTheOpenBatch(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		if inMemoryFixture(t) {
			// database/sql discards the connection of a transaction whose context is canceled, and
			// an in-memory SQLite database dies with its only connection.
			t.Skip("needs a database that outlives its connection")
		}
		c := groupcommit.New(db, logging.NewTestLogger(t), defs.GroupCommit{Enabled: true}, txScope)

		b := newBlocker()
		soloRan := make(chan struct{}, 1)
		result := make(chan error, 1)
		go func() {
			result <- c.Do(context.Background(), b.work("stuck"), func(ctx context.Context) error {
				soloRan <- struct{}{}
				return soloInsert(db, "stuck-solo")(ctx)
			})
		}()
		<-b.started

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		closed := make(chan error, 1)
		go func() { closed <- c.Close(ctx) }()

		// Close waits for the running unit even past its deadline; the batch it belongs to has been
		// canceled underneath it, so the unit's write fails and the caller falls back to solo.
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond) // let Close cancel the batch after its deadline
		close(b.release)

		require.ErrorIs(t, <-closed, context.DeadlineExceeded)
		require.NoError(t, <-result)
		assert.Len(t, soloRan, 1)
		assert.Equal(t, []string{"stuck-solo"}, callers(t, db))
		requireNoCommitterGoroutines(t)
	})
}

func TestCommitter_MaxWaitGathersABatch(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := newCommitter(t, db, defs.GroupCommit{Enabled: true, MaxBatch: 4, MaxWait: 2 * time.Second})

		var wg sync.WaitGroup
		start := time.Now()
		for i := range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				caller := fmt.Sprintf("c%d", i)
				assert.NoError(t, c.Do(context.Background(), insert(caller), soloInsert(db, caller)))
			}()
		}
		wg.Wait()

		batches, units := c.Stats()
		assert.Equal(t, uint64(1), batches, "the first unit waits for the batch to fill")
		assert.Equal(t, uint64(4), units)
		assert.Less(t, time.Since(start), 2*time.Second, "a full batch starts without waiting out MaxWait")
	})
}

func TestCommitter_CanceledContextRunsSoloWithoutQueueing(t *testing.T) {
	testDBs(t, func(t *testing.T, db *gorm.DB) {
		c := newCommitter(t, db, defs.GroupCommit{Enabled: true})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := c.Do(ctx, func(context.Context, *gorm.DB) error {
			return errors.New("work must not run")
		}, soloInsert(db, "x"))

		require.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, callers(t, db))
	})
}

// TestCommitter_DeferredSQLiteBatchWithAStaleSnapshotRunsSolo covers SQLite without
// _txlock=immediate: the batch transaction starts as a reader, another connection commits, and the
// batch's first write then fails with "database is locked". ROLLBACK TO cannot refresh the snapshot,
// so every unit of the batch must fall back to its own transaction instead of failing.
func TestCommitter_DeferredSQLiteBatchWithAStaleSnapshotRunsSolo(t *testing.T) {
	cfg := defs.DefaultDBConfig()
	cfg.SQLite.ConnectionString = "file:" + filepath.Join(t.TempDir(), "deferred.sqlite") // no _txlock
	cfg.MaxOpenConnections = 4
	cfg.MaxIdleConnections = 4
	database, err := database.NewDatabase(cfg, logging.NewTestLogger(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	db := database.DB
	require.NoError(t, db.AutoMigrate(&gcRow{}))

	c := newCommitter(t, db, defs.GroupCommit{Enabled: true})

	// The reader and its two mates queue up behind a blocker, so they share the next batch.
	var blockerWG sync.WaitGroup
	b := newBlocker()
	queueBehind(t, c, db, b, &blockerWG)

	started := make(chan struct{})
	release := make(chan struct{})
	readerErr := make(chan error, 1)
	go func() {
		readerErr <- c.Do(context.Background(), func(ctx context.Context, tx *gorm.DB) error {
			var n int64
			if err := tx.WithContext(ctx).Model(&gcRow{}).Count(&n).Error; err != nil { // takes the read snapshot
				return err
			}
			close(started)
			<-release
			return insert("reader")(ctx, tx)
		}, soloInsert(db, "reader"))
	}()
	waitQueued(t, c, 1)

	var wg sync.WaitGroup
	mateErrs := make([]error, 2)
	for i := range mateErrs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			caller := fmt.Sprintf("mate%d", i)
			mateErrs[i] = c.Do(context.Background(), insert(caller), soloInsert(db, caller))
		}()
		waitQueued(t, c, 2+i)
	}

	close(b.release)
	blockerWG.Wait()
	<-started

	// Another connection commits while the batch holds its (now stale) read snapshot.
	require.NoError(t, db.Create(&gcRow{Caller: "outside"}).Error)

	close(release)
	wg.Wait()

	for i, err := range mateErrs {
		require.NoError(t, err, "mate %d must not be failed by the stale batch", i)
	}
	require.NoError(t, <-readerErr)
	assert.Equal(t, []string{"blocker", "mate0", "mate1", "outside", "reader"}, callers(t, db))
	batches, units := c.Stats()
	assert.Equal(t, uint64(1), batches, "only the blocker's batch committed; the stale one was rolled back")
	assert.Equal(t, uint64(1), units)
}
