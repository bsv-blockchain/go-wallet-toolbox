package defs

import (
	"fmt"
	"time"
)

// DBType represents supported database types
type DBType string

// Supported database types
const (
	DBTypeMySQL    DBType = "mysql"
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
)

// ParseDBTypeStr parses a string to a DBType or returns an error
func ParseDBTypeStr(dbType string) (DBType, error) {
	return parseEnumCaseInsensitive(dbType, DBTypeMySQL, DBTypeSQLite, DBTypePostgres)
}

// Defaults for database configuration
const (
	DSNDefault         = "./storage.sqlite" // DSN for connection (file or memory, default is memory)
	DefaultTablePrefix = "bsv_"
)

// Database is a struct that configures the database connection
type Database struct {
	// Engine is the database engine (PostgreSQL, SQLite)
	Engine DBType `mapstructure:"engine"`

	// SQLite is configuration struct for SQLite database
	SQLite SQLite `mapstructure:"sqlite"`

	// PostgreSQL is configuration for PostgreSQL databases
	PostgreSQL PostgreSQL `mapstructure:"postgresql"`

	// MySQL is configuration for MySQL databases
	MySQL MySQL `mapstructure:"mysql"`

	// MaxIdleConnections defines the maximum number of idle connections allowed for the database.
	MaxIdleConnections int `mapstructure:"max_idle_connections"`

	// MaxConnectionIdleTime sets the maximum duration an idle connection can remain open before being closed.
	// Typically set in seconds.
	MaxConnectionIdleTime time.Duration `mapstructure:"max_connection_idle_time"`

	// MaxConnectionTime defines the maximum amount of time a connection may be reused.
	// Typically set in seconds.
	MaxConnectionTime time.Duration `mapstructure:"max_connection_time"`

	// MaxOpenConnections specifies the maximum number of open connections to the database.
	MaxOpenConnections int `mapstructure:"max_open_connections"`

	// PrepareStatements makes GORM prepare each distinct SQL text once per connection and reuse it
	// (gorm.Config.PrepareStmt). Default false. Mostly useful on SQLite and on Postgres, where the
	// driver runs in simple-protocol mode and never prepares by itself. The statement cache is
	// bounded (see DBPreparedStatementCacheSize).
	PrepareStatements bool `mapstructure:"prepare_statements"`

	// InsertBatchSize caps the number of rows per INSERT statement for multi-row inserts
	// (gorm.Config.CreateBatchSize). 0 (default) keeps the current behaviour: slice inserts go out
	// as one statement and the bulk repository helpers insert row by row. A value > 0 splits
	// slice inserts into statements of at most that many rows and lets the batched repository
	// helpers use them. It also applies to every other slice or association insert on the
	// connection.
	InsertBatchSize int `mapstructure:"insert_batch_size"`
}

// DBPreparedStatementCacheSize bounds the GORM prepared statement cache when PrepareStatements is on.
// GORM keys the cache by SQL text, so variable-length IN lists and variable-row inserts create one
// entry each; the bound keeps the cache (and the server-side statements) from growing without limit.
const DBPreparedStatementCacheSize = 1000

// SQLite is configuration struct for SQLite database
type SQLite struct {
	// ConnectionString is the path to SQLite DB
	ConnectionString string `mapstructure:"connection_string"`
}

// PostgreSQL is configuration struct for PostgreSQL database
type PostgreSQL struct {
	SQLCommon `mapstructure:",squash"`

	// ssl mode  [disable|allow|prefer|require|verify-ca|verify-full]. Will default to disable if not provided
	SslMode string `mapstructure:"ssl_mode"`

	// Schema allows separating instances within a single Postgres DB (default is empty/public)
	Schema string `mapstructure:"schema"`
}

// MySQL is configuration struct for MySQL database
type MySQL struct {
	SQLCommon `mapstructure:",squash"`

	// protocol for database connection [tcp|socket|pipe|memory]. Will default to tcp if not provided
	Protocol string `mapstructure:"protocol"`
}

// SQLCommon is configuration struct for common properties for SQL databases such as postgres or mysql
type SQLCommon struct {
	Host     string `mapstructure:"host"`
	DBName   string `mapstructure:"db_name"`
	Password string `mapstructure:"password"`
	Port     string `mapstructure:"port"`
	TimeZone string `mapstructure:"time_zone"`
	User     string `mapstructure:"user"`
}

// DefaultDBConfig sets default configuration for the database
func DefaultDBConfig() Database {
	return Database{
		Engine:                DBTypeSQLite,
		SQLite:                SQLite{ConnectionString: DSNDefault},
		MaxIdleConnections:    5,
		MaxConnectionIdleTime: 360 * time.Second,
		MaxConnectionTime:     60 * time.Second,
		MaxOpenConnections:    5,
		PostgreSQL: PostgreSQL{
			SslMode: "verify-full",
			SQLCommon: SQLCommon{
				Host:     "localhost",
				DBName:   "storage",
				User:     "postgres",
				Password: "<set-via-secret>",
				Port:     "5432",
				TimeZone: "UTC",
			},
		},
		MySQL: MySQL{
			Protocol: "tcp",
			SQLCommon: SQLCommon{
				Host:     "localhost",
				DBName:   "storage",
				User:     "root",
				Password: "",
				Port:     "3306",
				TimeZone: "UTC",
			},
		},
	}
}

// Validate validates if database configuration is valid
func (db *Database) Validate() (err error) {
	if db.Engine, err = ParseDBTypeStr(string(db.Engine)); err != nil {
		return fmt.Errorf("invalid DB engine: %w", err)
	}

	if db.InsertBatchSize < 0 {
		return fmt.Errorf("invalid DB insert batch size %d: must not be negative", db.InsertBatchSize)
	}

	return nil
}
