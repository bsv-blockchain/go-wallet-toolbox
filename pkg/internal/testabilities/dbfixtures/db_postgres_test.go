package dbfixtures_test

import (
	"net"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testmode"
	"github.com/stretchr/testify/require"
)

// skipIfPostgresUnavailable guards the fixture self-test so it doesn't fail
// hard in environments without a local Postgres running. It only probes TCP
// reachability (no SQL driver needed here — dbfixtures.TestDatabase below
// does the real connect through gorm's postgres dialector).
func skipIfPostgresUnavailable(t *testing.T) {
	t.Helper()

	dialer := &net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := dialer.DialContext(t.Context(), "tcp", "localhost:5432")
	if err != nil {
		t.Skip("postgres not available: start with `docker compose up -d db`")
		return
	}
	_ = conn.Close()
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
