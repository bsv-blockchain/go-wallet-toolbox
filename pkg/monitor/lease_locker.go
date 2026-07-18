package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// defaultLeaseTTL is used for a job whose TTL has not been set explicitly via
// SetLeaseTTL. Daemon.Start wires a per-job TTL of max(2*interval, 5m); this
// default only matters for lockers driven outside that path.
const defaultLeaseTTL = 10 * time.Minute

// monitorJobLock is one row per job name. Ownership of a job is expressed as
// holding an unexpired lease: a row whose Owner is this instance and whose
// LeaseUntil is in the future. Reclaim after a crash is automatic — once
// LeaseUntil is in the past, any instance may take the lease over.
//
// Clock model: leases are compared against the acquiring instance's own
// wall clock (UTC). The clock-skew tolerance is therefore the TTL slack, so
// deployed pods must be NTP-sane within minutes of each other. This is a much
// weaker assumption than a shared clock and is satisfied by any normal fleet.
type monitorJobLock struct {
	JobName    string    `gorm:"primaryKey"`
	Owner      string    `gorm:"not null"`
	LeaseUntil time.Time `gorm:"not null;index"`
	UpdatedAt  time.Time
}

// LeaseLocker is a gocron.Locker backed by a lease row per job in a shared
// database. Unlike a wall-clock-bucket identifier (which makes independent
// pods use different keys and therefore never contend), every instance
// contends on the SAME stable key per job, so exactly one instance runs a job
// at a time and a crashed owner is reclaimed after the lease expires.
type LeaseLocker struct {
	db     *gorm.DB
	owner  string
	logger *slog.Logger

	mu   sync.RWMutex
	ttls map[string]time.Duration
}

// NewLeaseLocker creates a LeaseLocker for the given owner and AutoMigrates its
// backing table. The owner must be unique per running instance (a random
// worker name is used by NewDaemonWithGORMLocker).
func NewLeaseLocker(db *gorm.DB, owner string, logger *slog.Logger) (*LeaseLocker, error) {
	if err := db.AutoMigrate(&monitorJobLock{}); err != nil {
		return nil, fmt.Errorf("failed to migrate monitor job lock table: %w", err)
	}

	return &LeaseLocker{
		db:     db,
		owner:  owner,
		logger: logging.Child(logger, "lease_locker"),
		ttls:   make(map[string]time.Duration),
	}, nil
}

// SetLeaseTTL sets the lease duration for a specific job key. It is called from
// Daemon.Start once task intervals are known. The key must match the gocron job
// name used as the lock key (see monitorJobName).
func (l *LeaseLocker) SetLeaseTTL(jobName string, ttl time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ttls[jobName] = ttl
}

func (l *LeaseLocker) ttlFor(jobName string) time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if ttl, ok := l.ttls[jobName]; ok && ttl > 0 {
		return ttl
	}
	return defaultLeaseTTL
}

// Lock attempts to acquire the lease for key. It succeeds when either no row
// exists, the existing lease has expired, or this instance already owns the
// lease (gocron never overlaps runs of one job within a single process, so
// same-owner re-acquire is the normal renewal path; the lease guards ACROSS
// processes). When another instance holds an unexpired lease, Lock returns an
// error — gocron then skips this run and reschedules — and logs the skip with
// the job name.
//
// The claim is a portable two-step: an OnConflict-DoNothing INSERT to ensure
// the row exists, then a CAS UPDATE guarded by
// `owner = me OR lease_until < now`. RowsAffected on the UPDATE is the sole
// arbiter, so it is correct even when two instances INSERT the row
// concurrently (the loser's CAS sees a foreign, unexpired lease and matches 0
// rows).
func (l *LeaseLocker) Lock(ctx context.Context, key string) (gocron.Lock, error) {
	now := time.Now().UTC()
	leaseUntil := now.Add(l.ttlFor(key))

	// Ensure a row exists. DoNothing on conflict keeps this portable across
	// SQLite / Postgres / MySQL and never clobbers a live lease held by
	// another instance.
	if err := l.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&monitorJobLock{JobName: key, Owner: l.owner, LeaseUntil: leaseUntil, UpdatedAt: now}).Error; err != nil {
		return nil, fmt.Errorf("lease ensure-row failed for %s: %w", key, err)
	}

	// Claim: take the lease iff we already own it or it has expired. UpdatedAt
	// is set explicitly to the same UTC clock as LeaseUntil so every column
	// this locker writes is UTC (gorm's auto-managed UpdatedAt would otherwise
	// use the process-local time.Now()).
	res := l.db.WithContext(ctx).
		Model(&monitorJobLock{}).
		Where("job_name = ? AND (owner = ? OR lease_until < ?)", key, l.owner, now).
		Updates(map[string]any{"owner": l.owner, "lease_until": leaseUntil, "updated_at": now})
	if res.Error != nil {
		return nil, fmt.Errorf("lease claim failed for %s: %w", key, res.Error)
	}
	if res.RowsAffected == 0 {
		l.logger.WarnContext(ctx, "monitor job skipped: lease held by another instance",
			slog.String("job", key))
		return nil, fmt.Errorf("lease for %s held by another instance", key)
	}

	return &leaseHandle{locker: l, key: key}, nil
}

// leaseHandle is the gocron.Lock returned on a successful claim.
type leaseHandle struct {
	locker *LeaseLocker
	key    string
}

// Unlock releases the lease early by expiring it, but only if this instance
// still owns it (a slow run whose lease already expired and was reclaimed must
// not stomp the new owner). gocron defers Unlock after every run, so this is
// the normal end-of-run release; the TTL is the crash-safety backstop.
func (h *leaseHandle) Unlock(ctx context.Context) error {
	now := time.Now().UTC()
	res := h.locker.db.WithContext(ctx).
		Model(&monitorJobLock{}).
		Where("job_name = ? AND owner = ?", h.key, h.locker.owner).
		Updates(map[string]any{"lease_until": now, "updated_at": now})
	if res.Error != nil {
		return fmt.Errorf("lease release failed for %s: %w", h.key, res.Error)
	}
	return nil
}
