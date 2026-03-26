package dbfixtures

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testmode"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
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

type DBConfigModifier func(config *defs.Database)

// TestDatabase creates a new database component, migrates database to make it ready for tests.
func TestDatabase(t testing.TB, configModifiers ...DBConfigModifier) (db *database.Database, cleanup func()) {
	dbConfig := DBConfigForTests()
	for _, modifier := range configModifiers {
		modifier(&dbConfig)
	}

	logger := logging.NewTestLogger(t)
	db, err := database.NewDatabase(dbConfig, logger)
	require.NoError(t, err)
	repos := db.CreateRepositories()
	err = repos.Migrate(t.Context())
	require.NoError(t, err)
	return db, func() {}
}

const sqliteFileNamePattern = `^file:(.+)\.sqlite(.*)$`

func WithSQLiteFileName(fileName string) DBConfigModifier {
	return func(config *defs.Database) {
		if config.Engine != defs.DBTypeSQLite {
			panic("WithSQLiteFileName modifier can only be used with SQLite engine (config.Engine = 'sqlite')")
		}
		re := regexp.MustCompile(sqliteFileNamePattern)
		config.SQLite.ConnectionString = re.ReplaceAllString(
			config.SQLite.ConnectionString,
			"file:"+fileName+".sqlite$2",
		)
	}
}
