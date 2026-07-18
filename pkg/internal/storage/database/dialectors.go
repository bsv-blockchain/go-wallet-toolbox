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
// WAL journal mode and a 5s busy_timeout — to dsn, filling in each pragma
// independently and only when dsn does not already set it. In-memory databases
// are left untouched (WAL is meaningless without a file).
//
// WAL lets readers and a writer proceed concurrently instead of the default
// rollback-journal mode's single-writer-excludes-all-readers behavior. The
// busy_timeout matches mattn/go-sqlite3's built-in 5000ms default — pinning it
// in the DSN makes the value explicit and survives any future driver-default
// change; WAL is the actual behavior change here.
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
// Each pragma is defaulted independently: an operator who pins only
// _busy_timeout still receives the WAL default (and vice versa). Any pragma the
// DSN already sets is left untouched, so explicit operator values always win.
func sqliteDSNWithDefaults(dsn string) string {
	// In-memory databases have no file, so WAL is meaningless and these pragmas
	// don't apply — leave the DSN untouched.
	if strings.Contains(dsn, ":memory:") || strings.Contains(dsn, "mode=memory") {
		return dsn
	}

	var missing []string
	if !strings.Contains(dsn, "_journal_mode") {
		missing = append(missing, "_journal_mode=WAL")
	}
	if !strings.Contains(dsn, "_busy_timeout") {
		missing = append(missing, "_busy_timeout=5000")
	}
	if len(missing) == 0 {
		return dsn
	}

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}

	return dsn + sep + strings.Join(missing, "&")
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
