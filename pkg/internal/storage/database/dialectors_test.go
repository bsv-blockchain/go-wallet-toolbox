package database

import (
	"path/filepath"
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
			want: "./storage.sqlite?_journal_mode=WAL&_busy_timeout=5000",
		},
		"file: path with existing query gets defaults appended with an ampersand": {
			dsn:  "file:x.db?cache=shared",
			want: "file:x.db?cache=shared&_journal_mode=WAL&_busy_timeout=5000",
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
		"existing _journal_mode is left unchanged": {
			dsn:  "file:x.db?_journal_mode=DELETE",
			want: "file:x.db?_journal_mode=DELETE",
		},
		"existing _busy_timeout is left unchanged": {
			dsn:  "file:x.db?_busy_timeout=100",
			want: "file:x.db?_busy_timeout=100",
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
