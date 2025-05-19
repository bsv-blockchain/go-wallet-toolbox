package dbfixtures

import (
	"context"
	"testing"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/database"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/testabilities/testmode"
	"github.com/stretchr/testify/require"
)

func DBConfigForTests() defs.Database {
	dbConfig := defs.DefaultDBConfig()
	dbConfig.MaxIdleConnections = 1
	dbConfig.MaxOpenConnections = 1

	switch mode := testmode.GetMode().(type) {
	case *testmode.SQLiteFileMode:
		{
			dbConfig.SQLite.ConnectionString = mode.ConnectionString
		}
	case *testmode.PostgresMode:
		{
			dbConfig.Engine = defs.DBTypePostgres
			dbConfig.PostgreSQL.DBName = mode.DBName
			dbConfig.PostgreSQL.Host = mode.Host
			dbConfig.PostgreSQL.User = mode.User
			dbConfig.PostgreSQL.Password = mode.Password
		}
	default:
		{
			dbConfig.SQLite.ConnectionString = "file:storage.test.sqlite?mode=memory"
		}
	}
	return dbConfig
}

// TestDatabase creates a new database component, migrates database to make it ready for tests.
func TestDatabase(t testing.TB) (db *database.Database, cleanup func()) {
	dbConfig := DBConfigForTests()
	logger := logging.NewTestLogger(t)
	db, err := database.NewDatabase(dbConfig, logger)
	require.NoError(t, err)
	repos := db.CreateRepositories()
	err = repos.Migrate(context.Background())
	require.NoError(t, err)
	return db, func() {}
}
