package gormstorage

import (
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// DefaultSQLiteInMemoryConnectionStr is the connection string for a shared in-memory SQLite database instance.
const DefaultSQLiteInMemoryConnectionStr = "file::memory:?cache=shared"

// InMemorySQLiteDBConfig returns a Database config set up for an in-memory SQLite database instance.
func InMemorySQLiteDBConfig() defs.Database {
	return defs.Database{
		Engine:                defs.DBTypeSQLite,
		SQLite:                defs.SQLite{ConnectionString: DefaultSQLiteInMemoryConnectionStr},
		MaxIdleConnections:    5,
		MaxConnectionIdleTime: 0, // set to 0 to prevent the pool from ever closing idle connections.
		MaxConnectionTime:     60 * time.Second,
		MaxOpenConnections:    5,
	}
}
