package dbfixtures

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
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
	// In-memory SQLite databases live only as long as their backing connection.
	// Keep the single test connection alive for long integration tests.
	dbConfig.MaxConnectionIdleTime = 0
	dbConfig.MaxConnectionTime = 0

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
			dbConfig.PostgreSQL.SslMode = "disable" // local/CI test postgres runs without TLS
			// concurrency tests need real parallelism; the 1-conn pin is SQLite-only
			dbConfig.MaxIdleConnections = 10
			dbConfig.MaxOpenConnections = 10
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
// Under TEST_DB_MODE=postgres, each call gets its own Postgres schema (isolated from other
// concurrently running tests) which is dropped by the returned cleanup func.
func TestDatabase(t testing.TB, configModifiers ...DBConfigModifier) (db *database.Database, cleanup func()) {
	dbConfig := DBConfigForTests()
	for _, modifier := range configModifiers {
		modifier(&dbConfig)
	}

	var schema string
	if dbConfig.Engine == defs.DBTypePostgres {
		schema = testSchemaName(t)
		dbConfig.PostgreSQL.Schema = schema
	}

	logger := logging.NewTestLogger(t)
	db, err := database.NewDatabase(dbConfig, logger)
	require.NoError(t, err)
	repos := db.CreateRepositories()
	err = repos.Migrate(t.Context())
	require.NoError(t, err)

	cleanup = func() {
		// no-op: the default in-memory SQLite database vanishes with its
		// connection; only the Postgres path below needs explicit teardown.
	}
	if schema != "" {
		cleanup = func() {
			// Drop the schema first, then close the pool: with hundreds of
			// tests sharing one Postgres server, leaving connections open
			// for the lifetime of the test binary exhausts max_connections.
			_ = db.DB.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema)).Error
			_ = db.Close()
		}
	}
	return db, cleanup
}

// testSchemaName builds a valid, unique postgres schema name (<=63 chars) from the test name.
func testSchemaName(t testing.TB) string {
	sanitized := strings.ToLower(regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(strings.ToLower(t.Name()), "_"))
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	name := fmt.Sprintf("t_%s_%x", sanitized, suffix)
	if len(name) > 63 {
		name = name[:50] + name[len(name)-13:]
	}
	return name
}

const sqliteFileNamePattern = `^file:(.+)\.sqlite(.*)$`

// WithSQLiteFileName renames the SQLite file backing the test database.
// It is a no-op when the resolved engine is not SQLite (e.g. TEST_DB_MODE=postgres),
// since each TestDatabase call already gets its own isolated schema in that mode.
func WithSQLiteFileName(fileName string) DBConfigModifier {
	return func(config *defs.Database) {
		if config.Engine != defs.DBTypeSQLite {
			return
		}
		re := regexp.MustCompile(sqliteFileNamePattern)
		config.SQLite.ConnectionString = re.ReplaceAllString(
			config.SQLite.ConnectionString,
			"file:"+fileName+".sqlite$2",
		)
	}
}
