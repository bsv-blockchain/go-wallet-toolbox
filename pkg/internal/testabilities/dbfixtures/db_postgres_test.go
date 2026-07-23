package dbfixtures_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testmode"
)

// skipIfPostgresUnavailable guards the fixture self-test so it doesn't fail
// hard in environments without the expected local Postgres running. It
// authenticates with the same credentials dbfixtures.TestDatabase below uses,
// rather than merely probing TCP reachability: some dev machines run an
// unrelated Postgres on 5432 (different role/password), which would otherwise
// be mistaken for the docker-compose test instance and fail with an auth error.
func skipIfPostgresUnavailable(t *testing.T) {
	t.Helper()

	db, err := sql.Open("pgx", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	if err != nil {
		t.Skip("postgres not available: start with `docker compose up -d db`")
		return
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skip("postgres not available: start with `docker compose up -d db`")
	}
}

// Requires: docker compose up -d db
func TestPostgresFixtureIsolation(t *testing.T) {
	skipIfPostgresUnavailable(t)
	testmode.DevelopmentOnly_SetPostgresMode(t)

	// given: two independent test databases in the same run
	db1, cleanup1 := dbfixtures.TestDatabase(t)
	db2, cleanup2 := dbfixtures.TestDatabase(t)
	defer cleanup1()
	defer cleanup2()

	// when: a row is written through db1
	require.NoError(t, db1.DB.Exec(`INSERT INTO bsv_users (identity_key, active_storage, created_at, updated_at) VALUES ('isolation-probe', 'x', now(), now())`).Error)

	// then: db2 does not see it (schema isolation)
	var count int64
	require.NoError(t, db2.DB.Raw(`SELECT count(*) FROM bsv_users WHERE identity_key = 'isolation-probe'`).Scan(&count).Error)
	require.Zero(t, count)

	// and: the pool is not serialized to one connection
	sqlDB, err := db1.DB.DB()
	require.NoError(t, err)
	require.Greater(t, sqlDB.Stats().MaxOpenConnections, 1)
}
