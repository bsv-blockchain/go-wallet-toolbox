package database

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/sqlite3extended"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

	return sqlite.New(sqlite.Config{
		Conn:       nil,
		DriverName: sqlite3extended.NAME,
		DSN:        dsn,
	})
}

func postgresDialector(cfg defs.Database) gorm.Dialector {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		cfg.PostgreSQL.Host, cfg.PostgreSQL.User, cfg.PostgreSQL.Password, cfg.PostgreSQL.DBName,
		cfg.PostgreSQL.Port, cfg.PostgreSQL.SslMode, cfg.PostgreSQL.TimeZone,
	)
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
	dsn := fmt.Sprintf("%s:%s@%s(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=%s",
		cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.Protocol, cfg.MySQL.Host,
		cfg.MySQL.Port, cfg.MySQL.DBName, normalizeTimeZone(cfg.MySQL.TimeZone),
	)
	// potentially use null as default
	return mysql.New(mysql.Config{
		DSN:  dsn,
		Conn: nil,
	})
}
