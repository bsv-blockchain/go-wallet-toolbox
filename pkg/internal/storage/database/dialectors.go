package database

import (
	"fmt"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/sqlite3extended"
)

type dialectorMaker func(cfg defs.Database) gorm.Dialector

var dialectors = map[defs.DBType]dialectorMaker{
	defs.DBTypeSQLite:   sqliteDialector,
	defs.DBTypePostgres: postgresDialector,
	defs.DBTypeMySQL:    mysqlDialector,
}

func sqliteDialector(cfg defs.Database) gorm.Dialector {
	dsn := cfg.SQLite.ConnectionString
	if dsn == "" {
		dsn = defs.DSNDefault
	}
	dsn = sqliteDSNWithDefaults(dsn)

	return sqlite.New(sqlite.Config{
		Conn:       nil,
		DriverName: sqlite3extended.NAME,
		DSN:        dsn,
	})
}

// sqliteDSNWithDefaults appends operator-friendly SQLite pragma defaults —
// WAL journal mode and a 5s busy_timeout — to dsn, unless dsn is an in-memory
// database or already configures either pragma explicitly.
//
// WAL lets readers and a writer proceed concurrently instead of the default
// rollback-journal mode's single-writer-excludes-all-readers behavior, and the
// busy_timeout gives concurrent writers (which WAL still serializes) a window
// to wait for the SQLITE_BUSY lock to clear instead of failing immediately.
// This matters because gorm's sqlite dialector passes the DSN to sql.Open
// unmodified (gorm.io/driver/sqlite Dialector.Initialize), and mattn/go-sqlite3
// parses driver params (_journal_mode, _busy_timeout, ...) from the query
// string after '?' regardless of whether the DSN carries a "file:" prefix —
// see (*SQLiteDriver).Open in github.com/mattn/go-sqlite3, which splits dsn on
// the first '?', runs url.ParseQuery on the remainder, and — only for DSNs
// *without* a "file:" prefix — then strips the query string back off before
// handing the bare path to sqlite3_open_v2. So appending params to a bare
// path (no "file:" prefix) works exactly like appending them to a "file:" DSN.
//
// Operator-provided params always win: this only fills in defaults when the
// DSN is silent on both pragmas, so any explicit _journal_mode or
// _busy_timeout (or an in-memory DSN, where WAL is meaningless and disk-only)
// is left untouched.
func sqliteDSNWithDefaults(dsn string) string {
	switch {
	case strings.Contains(dsn, ":memory:"),
		strings.Contains(dsn, "mode=memory"),
		strings.Contains(dsn, "_journal_mode"),
		strings.Contains(dsn, "_busy_timeout"):
		return dsn
	}

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}

	return dsn + sep + "_journal_mode=WAL&_busy_timeout=5000"
}

func postgresDialector(cfg defs.Database) gorm.Dialector {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		cfg.PostgreSQL.Host, cfg.PostgreSQL.User, cfg.PostgreSQL.Password, cfg.PostgreSQL.DBName,
		cfg.PostgreSQL.Port, cfg.PostgreSQL.SslMode, cfg.PostgreSQL.TimeZone,
	)

	if cfg.PostgreSQL.Schema != "" {
		dsn = fmt.Sprintf("%s search_path=%s", dsn, cfg.PostgreSQL.Schema)
	}

	return postgres.New(postgres.Config{
		PreferSimpleProtocol: true, // turn to TRUE to disable implicit prepared statement usage
		WithoutReturning:     false,
		DSN:                  dsn,
	})
}

func mysqlDialector(cfg defs.Database) gorm.Dialector {
	// parseTime=True is required for the db to be able to parse time correctly
	// charset=utf8mb4 is required for the db to parse utf-8 encoding properly
	// please refer to: https://gorm.io/docs/connecting_to_the_database.html#MySQL
	dsn := fmt.Sprintf(
		"%s:%s@%s(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=%s",
		cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.Protocol, cfg.MySQL.Host,
		cfg.MySQL.Port, cfg.MySQL.DBName, normalizeTimeZone(cfg.MySQL.TimeZone),
	)
	// potentially use null as default
	return mysql.New(mysql.Config{
		DSN:  dsn,
		Conn: nil,
	})
}
