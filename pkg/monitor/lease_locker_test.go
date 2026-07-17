package monitor_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/dbfixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
)

// newLockTestDB opens a *gorm.DB backed by a durable store that two lockers can
// share. Under the default (or TEST_DB_MODE=file) mode it is a file-backed
// SQLite database in t.TempDir(): the monitor fixture's per-connection
// `mode=memory` DB cannot be shared between two lockers, so a real file is
// required. Under TEST_DB_MODE=postgres each test gets its own schema.
func newLockTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := dbfixtures.DBConfigForTests()
	if cfg.Engine == defs.DBTypePostgres {
		cfg.PostgreSQL.Schema = lockTestSchema(t)
	} else { // SQLite (default or TEST_DB_MODE=file)
		cfg.Engine = defs.DBTypeSQLite
		cfg.SQLite.ConnectionString = "file:" + filepath.Join(t.TempDir(), "monitor_lock.sqlite")
	}

	db, err := database.NewDatabase(cfg, logging.NewTestLogger(t))
	require.NoError(t, err)

	t.Cleanup(func() {
		if cfg.Engine == defs.DBTypePostgres && cfg.PostgreSQL.Schema != "" {
			_ = db.DB.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, cfg.PostgreSQL.Schema)).Error
		}
		_ = db.Close()
	})

	return db.DB
}

func lockTestSchema(t *testing.T) string {
	t.Helper()
	sanitized := regexp.MustCompile(`[^a-z0-9_]+`).ReplaceAllString(strings.ToLower(t.Name()), "_")
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	name := fmt.Sprintf("t_%s_%x", sanitized, suffix)
	if len(name) > 63 {
		name = name[:50] + name[len(name)-13:]
	}
	return name
}

func newLocker(t *testing.T, db *gorm.DB, owner string) *monitor.LeaseLocker {
	t.Helper()
	l, err := monitor.NewLeaseLocker(db, owner, logging.NewTestLogger(t))
	require.NoError(t, err)
	return l
}

// TestLeaseLocker_AcquireAndContend: owner A acquires "job"; owner B (different
// owner, same DB) is refused while A's lease is live.
func TestLeaseLocker_AcquireAndContend(t *testing.T) {
	db := newLockTestDB(t)
	a := newLocker(t, db, "owner-A")
	b := newLocker(t, db, "owner-B")
	ctx := context.Background()

	lockA, err := a.Lock(ctx, "job")
	require.NoError(t, err, "owner A should acquire the free lease")
	require.NotNil(t, lockA)

	_, err = b.Lock(ctx, "job")
	require.Error(t, err, "owner B must be refused while A holds an unexpired lease")
}

// TestLeaseLocker_ReclaimAfterExpiry: A acquires with a 50ms TTL; after it
// expires, B reclaims the lease.
func TestLeaseLocker_ReclaimAfterExpiry(t *testing.T) {
	db := newLockTestDB(t)
	a := newLocker(t, db, "owner-A")
	b := newLocker(t, db, "owner-B")
	a.SetLeaseTTL("job", 50*time.Millisecond)
	b.SetLeaseTTL("job", 50*time.Millisecond)
	ctx := context.Background()

	_, err := a.Lock(ctx, "job")
	require.NoError(t, err, "owner A should acquire the free lease")

	time.Sleep(60 * time.Millisecond)

	_, err = b.Lock(ctx, "job")
	require.NoError(t, err, "owner B should reclaim the lease after it expires")
}

// TestLeaseLocker_UnlockReleases: A acquires then unlocks; B acquires immediately.
func TestLeaseLocker_UnlockReleases(t *testing.T) {
	db := newLockTestDB(t)
	a := newLocker(t, db, "owner-A")
	b := newLocker(t, db, "owner-B")
	ctx := context.Background()

	lockA, err := a.Lock(ctx, "job")
	require.NoError(t, err, "owner A should acquire the free lease")
	require.NoError(t, lockA.Unlock(ctx), "owner A should release its lease")

	_, err = b.Lock(ctx, "job")
	require.NoError(t, err, "owner B should acquire immediately after A releases")
}

// TestLeaseLocker_SameOwnerReacquires: the same owner re-acquires its own live
// lease (gocron never overlaps a single job in one process; the lease only
// guards across processes).
func TestLeaseLocker_SameOwnerReacquires(t *testing.T) {
	db := newLockTestDB(t)
	a := newLocker(t, db, "owner-A")
	ctx := context.Background()

	_, err := a.Lock(ctx, "job")
	require.NoError(t, err, "owner A should acquire the free lease")

	_, err = a.Lock(ctx, "job")
	require.NoError(t, err, "owner A should re-acquire its own live lease without unlocking")
}
