package database

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

func TestSqliteDSNWithDefaults(t *testing.T) {
	tests := map[string]struct {
		dsn  string
		want string
	}{
		"plain path gets defaults appended with a leading question mark": {
			dsn:  "./storage.sqlite",
			want: "./storage.sqlite?_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate",
		},
		"file: path with existing query gets defaults appended with an ampersand": {
			dsn:  "file:x.db?cache=shared",
			want: "file:x.db?cache=shared&_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate",
		},
		"bare :memory: is left unchanged": {
			dsn:  ":memory:",
			want: ":memory:",
		},
		"file::memory: with query is left unchanged": {
			dsn:  "file::memory:?cache=shared",
			want: "file::memory:?cache=shared",
		},
		"mode=memory query param is left unchanged even without the :memory: token": {
			dsn:  "file:x.db?cache=shared&mode=memory",
			want: "file:x.db?cache=shared&mode=memory",
		},
		"existing _journal_mode is preserved but the missing _busy_timeout is still added": {
			dsn:  "file:x.db?_journal_mode=DELETE",
			want: "file:x.db?_journal_mode=DELETE&_busy_timeout=5000&_txlock=immediate",
		},
		"existing _busy_timeout is preserved but the missing _journal_mode (WAL) is still added": {
			dsn:  "file:x.db?_busy_timeout=100",
			want: "file:x.db?_busy_timeout=100&_journal_mode=WAL&_txlock=immediate",
		},
		"both pragmas already set still get the missing _txlock": {
			dsn:  "file:x.db?_journal_mode=DELETE&_busy_timeout=100",
			want: "file:x.db?_journal_mode=DELETE&_busy_timeout=100&_txlock=immediate",
		},
		"existing _txlock is preserved": {
			dsn:  "file:x.db?_txlock=deferred",
			want: "file:x.db?_txlock=deferred&_journal_mode=WAL&_busy_timeout=5000",
		},
		"all params already set are left entirely unchanged": {
			dsn:  "file:x.db?_journal_mode=DELETE&_busy_timeout=100&_txlock=exclusive",
			want: "file:x.db?_journal_mode=DELETE&_busy_timeout=100&_txlock=exclusive",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := sqliteDSNWithDefaults(tt.dsn)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestSqliteDialector_WALDefaultsAppliedThroughFullPath opens a real temp-file
// SQLite database through the full sqliteDialector path (as NewDatabase does)
// and asserts that both pragmas the DSN massage is meant to guarantee are
// actually in effect on the live connection - not just present in the string.
//
// This also empirically verifies the mattn/go-sqlite3 param-parsing behavior
// documented on sqliteDSNWithDefaults: for a bare path DSN (no "file:" prefix),
// query params appended after '?' are still parsed and applied by the driver,
// even though the driver strips the query string back off before opening the
// underlying sqlite3 file handle.
func TestSqliteDialector_WALDefaultsAppliedThroughFullPath(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "wal_defaults.sqlite")

	cfg := defs.DefaultDBConfig()
	cfg.SQLite.ConnectionString = dsn

	db, err := NewDatabase(cfg, logging.NewTestLogger(t))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	var journalMode string
	require.NoError(t, db.DB.Raw("PRAGMA journal_mode;").Scan(&journalMode).Error)
	require.Equal(t, "wal", journalMode)

	var busyTimeout int
	require.NoError(t, db.DB.Raw("PRAGMA busy_timeout;").Scan(&busyTimeout).Error)
	require.Equal(t, 5000, busyTimeout)
}

// TestSqliteDialector_ConcurrentReadThenWriteTransactions runs concurrent
// transactions that read before they write, the shape of CreateAction. With
// deferred transactions SQLite fails the read-to-write lock upgrade at once
// with "database is locked"; the _txlock=immediate default makes every
// transaction wait on _busy_timeout instead, so all of them commit.
func TestSqliteDialector_ConcurrentReadThenWriteTransactions(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "txlock.sqlite")

	cfg := defs.DefaultDBConfig()
	cfg.SQLite.ConnectionString = dsn

	db, err := NewDatabase(cfg, logging.NewTestLogger(t))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	require.NoError(t, db.DB.Exec("CREATE TABLE counter (id INTEGER PRIMARY KEY, n INTEGER)").Error)
	require.NoError(t, db.DB.Exec("INSERT INTO counter (id, n) VALUES (1, 0)").Error)

	const workers, perWorker = 8, 25

	var wg sync.WaitGroup
	errs := make(chan error, workers*perWorker)

	for range workers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range perWorker {
				tx := db.DB.Begin()
				if tx.Error != nil {
					errs <- tx.Error
					continue
				}

				var n int
				if err := tx.Raw("SELECT n FROM counter WHERE id = 1").Scan(&n).Error; err != nil {
					tx.Rollback()
					errs <- err
					continue
				}

				if err := tx.Exec("UPDATE counter SET n = ? WHERE id = 1", n+1).Error; err != nil {
					tx.Rollback()
					errs <- err
					continue
				}

				errs <- tx.Commit().Error
			}
		}()
	}

	wg.Wait()
	close(errs)

	var failed []error
	for err := range errs {
		if err != nil {
			failed = append(failed, err)
		}
	}
	require.NoError(t, errors.Join(failed...))

	var n int
	require.NoError(t, db.DB.Raw("SELECT n FROM counter WHERE id = 1").Scan(&n).Error)
	require.Equal(t, workers*perWorker, n)
}
