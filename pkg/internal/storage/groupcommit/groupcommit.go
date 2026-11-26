// Package groupcommit runs units of database work from concurrent callers inside shared
// transactions ("group commit"). Each unit runs inside its own SAVEPOINT, so a failing unit is
// rolled back alone and its batch-mates still commit; every caller returns only after the shared
// transaction holding its work has committed.
//
// A unit that cannot be completed inside a batch - the engine aborted it to break a lock conflict,
// or the whole batch was rolled back - is handed back to its caller, which runs its own ungrouped
// code path ("solo"). Callers therefore see the same results and errors as without grouping.
package groupcommit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/dbretry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// Work is one caller's unit of database work. scope is built once per batch over the shared
// transaction (see New) and the unit runs positioned inside its own savepoint; work must use only
// the shared transaction, never another connection, and must not call out to the network. ctx
// carries the caller's values but never its cancellation: a canceled statement would abort the
// shared transaction. Work may run more than once (see Solo), so it must only write to the database
// and fully re-assign any state it captures.
type Work[S any] = func(ctx context.Context, scope S) error

// Solo is the caller's ungrouped code path for the same unit of work, run in the caller's
// goroutine with the caller's context when the work cannot be (or could not be) done in a batch.
type Solo = func(ctx context.Context) error

const (
	statePending int32 = iota
	stateRunning
	stateCanceled
)

type request[S any] struct {
	ctx   context.Context
	work  Work[S]
	state atomic.Int32
	done  chan outcome
}

type outcome struct {
	err      error
	solo     bool
	panicked bool
	panicVal any
}

// Committer batches units of work into shared transactions. Create it with New and stop it with
// Close. It is safe for concurrent use.
type Committer[S any] struct {
	db       *gorm.DB
	newScope func(tx *gorm.DB) S
	logger   *slog.Logger
	maxBatch int
	maxWait  time.Duration

	mu     sync.Mutex
	queue  []*request[S]
	closed bool

	wake chan struct{}
	stop chan struct{}

	// ctx bounds every batch transaction; Close cancels it only when its own deadline passes.
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	batches   atomic.Uint64
	committed atomic.Uint64
}

// New starts a Committer over db with cfg's limits (zero values select the defaults). newScope
// builds, once per batch, what the batch's units run on from the shared transaction - typically
// repositories bound to it - so the per-unit cost stays small.
func New[S any](db *gorm.DB, logger *slog.Logger, cfg defs.GroupCommit, newScope func(tx *gorm.DB) S) *Committer[S] {
	maxBatch := cfg.MaxBatch
	if maxBatch <= 0 {
		maxBatch = defs.DefaultGroupCommitMaxBatch
	}
	workers := cfg.Workers
	if workers <= 0 {
		workers = defs.DefaultGroupCommitWorkers
	}

	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // cancel is kept in the Committer and called by Close
	c := &Committer[S]{
		db:       db,
		newScope: newScope,
		logger:   logging.Child(logger, "groupCommit"),
		maxBatch: maxBatch,
		maxWait:  max(cfg.MaxWait, 0),
		wake:     make(chan struct{}, 1),
		stop:     make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
	}

	c.wg.Add(workers)
	for range workers {
		go c.run()
	}

	return c
}

// Do runs work inside a transaction shared with other callers' work and returns once that
// transaction has committed, with work's own error if it failed (its writes rolled back, its
// batch-mates unaffected). When the work cannot be done in a batch, Do runs solo instead.
//
// If ctx is done before the work started, Do runs solo with ctx, which fails the same way the
// ungrouped path fails on a canceled context. Once the work started, Do waits for its batch, so an
// error always means that nothing of this unit was committed.
func (c *Committer[S]) Do(ctx context.Context, work Work[S], solo Solo) error {
	if ctx.Err() != nil {
		return solo(ctx)
	}

	r := &request[S]{ctx: ctx, work: work, done: make(chan outcome, 1)}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return solo(ctx)
	}
	c.queue = append(c.queue, r)
	c.mu.Unlock()
	c.signal()

	var out outcome
	select {
	case out = <-r.done:
	case <-ctx.Done():
		if r.state.CompareAndSwap(statePending, stateCanceled) {
			return solo(ctx)
		}
		out = <-r.done
	}

	switch {
	case out.panicked:
		panic(out.panicVal)
	case out.solo:
		return solo(ctx)
	default:
		return out.err
	}
}

// Close stops accepting work (later Do calls run solo), lets the workers finish what is queued and
// waits for them. If ctx ends first, the open batch transaction is canceled - its units run solo,
// or fail if the cancellation hit the COMMIT - and Close still waits for the workers to exit
// before returning ctx's error. Close is idempotent.
func (c *Committer[S]) Close(ctx context.Context) error {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		close(c.stop)
	}
	c.mu.Unlock()

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.cancel()
		return nil
	case <-ctx.Done():
		c.cancel()
		<-done
		return fmt.Errorf("group commit did not drain before the deadline: %w", ctx.Err())
	}
}

// Stats reports how many batch transactions committed and how many units of work they carried.
func (c *Committer[S]) Stats() (batches, units uint64) {
	return c.batches.Load(), c.committed.Load()
}

// Queued reports how many units are waiting for a batch.
func (c *Committer[S]) Queued() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.queue)
}

func (c *Committer[S]) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Committer[S]) run() {
	defer c.wg.Done()
	for {
		batch, ok := c.next()
		if !ok {
			return
		}
		c.execute(batch)
	}
}

// next blocks until there is work and returns up to maxBatch queued requests, or false once the
// committer is closed and the queue is empty.
func (c *Committer[S]) next() ([]*request[S], bool) {
	for {
		c.mu.Lock()
		if n := len(c.queue); n > 0 && n < c.maxBatch && c.maxWait > 0 && !c.closed {
			c.mu.Unlock()
			c.gather()
			c.mu.Lock()
		}

		if n := min(len(c.queue), c.maxBatch); n > 0 {
			batch := make([]*request[S], n)
			copy(batch, c.queue)
			rest := copy(c.queue, c.queue[n:])
			clear(c.queue[rest:])
			c.queue = c.queue[:rest]
			c.mu.Unlock()

			if rest > 0 {
				c.signal() // let another worker (or this one, next round) take the rest
			}
			return batch, true
		}

		closed := c.closed
		c.mu.Unlock()
		if closed {
			return nil, false
		}

		select {
		case <-c.wake:
		case <-c.stop:
		}
	}
}

// gather waits up to maxWait for the queue to fill a batch.
func (c *Committer[S]) gather() {
	timer := time.NewTimer(c.maxWait)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return
		case <-c.stop:
			return
		case <-c.wake:
			c.mu.Lock()
			full := len(c.queue) >= c.maxBatch
			c.mu.Unlock()
			if full {
				return
			}
		}
	}
}

func (c *Committer[S]) execute(batch []*request[S]) {
	live := batch[:0]
	for _, r := range batch {
		if r.state.CompareAndSwap(statePending, stateRunning) {
			live = append(live, r)
		}
	}
	if len(live) == 0 {
		return
	}

	results := make([]outcome, len(live))
	if !c.runBatch(live, results) {
		for i := range results {
			if !results[i].panicked {
				results[i] = outcome{solo: true}
			}
		}
	}

	for i, r := range live {
		r.done <- results[i]
	}
}

// runBatch runs the requests in one transaction and fills results. It returns false when the
// transaction was rolled back as a whole, so the units must run solo.
func (c *Committer[S]) runBatch(live []*request[S], results []outcome) bool {
	tx := c.db.WithContext(c.ctx).Begin()
	if tx.Error != nil {
		c.logger.WarnContext(c.ctx, "group commit could not begin a transaction; running the batch ungrouped",
			slog.Int("units", len(live)), logging.Error(tx.Error))
		return false
	}

	scope := c.newScope(tx)
	for i, r := range live {
		savepoint := "gc" + strconv.Itoa(i)
		// Raw statements, not tx.SavePoint/RollbackTo: the SQLite and Postgres dialectors discard
		// the error of those, and a failed ROLLBACK TO is exactly what has to be detected here.
		if err := tx.Exec("SAVEPOINT " + savepoint).Error; err != nil {
			c.abort(tx, "savepoint failed", len(live), err)
			return false
		}

		panicked, panicVal, err := runUnit(r, scope)
		if err == nil && !panicked {
			continue
		}

		if rbErr := tx.Exec("ROLLBACK TO SAVEPOINT " + savepoint).Error; rbErr != nil {
			// The engine already rolled the whole transaction back (or the connection is gone).
			c.abort(tx, "rollback to savepoint failed", len(live), rbErr)
			if panicked {
				results[i] = outcome{panicked: true, panicVal: panicVal}
			}
			return false
		}

		switch {
		case panicked:
			results[i] = outcome{panicked: true, panicVal: panicVal}
		case sqliteBusy(err):
			// SQLite without _txlock=immediate begins the batch as a reader; if another
			// connection committed since, the batch's first write fails with "database is
			// locked" and every later unit's would too, because ROLLBACK TO does not refresh the
			// outer read snapshot. Rolling the whole batch back and running its units on their
			// own gives each the fresh transaction it would have had without grouping.
			c.abort(tx, "sqlite busy", len(live), err)
			return false
		case dbretry.Retryable(err):
			// The engine picked this unit to break a lock conflict. Retrying inside the batch would
			// meet the same cycle (the batch still holds the earlier units' locks), so the unit runs
			// its own retrying path once the batch is done.
			results[i] = outcome{solo: true}
		default:
			results[i] = outcome{err: err}
		}
	}

	if err := c.ctx.Err(); err != nil {
		// Close gave up waiting: do not start a COMMIT that the cancellation could interrupt.
		c.abort(tx, "committer closed", len(live), err)
		return false
	}

	if err := tx.Commit().Error; err != nil {
		if commitRolledBack(err) {
			c.logger.WarnContext(c.ctx, "group commit transaction was rolled back at commit; running the batch ungrouped",
				slog.Int("units", len(live)), logging.Error(err))
			return false
		}
		// The outcome of a failed COMMIT is not known (it may have committed), so the units are not
		// re-run: that could apply them twice. Each caller gets the error, as it would from its own
		// COMMIT.
		c.logger.ErrorContext(c.ctx, "group commit transaction failed to commit", slog.Int("units", len(live)), logging.Error(err))
		for i := range results {
			if results[i].err == nil && !results[i].solo && !results[i].panicked {
				results[i] = outcome{err: fmt.Errorf("failed to commit group transaction: %w", err)}
			}
		}
		return true
	}

	c.batches.Add(1)
	c.committed.Add(uint64(len(live)))
	return true
}

// commitRolledBack reports whether a COMMIT error means the transaction certainly did not commit:
// the engine rolled it back to break a conflict, or database/sql had already rolled it back
// (ErrTxDone). A context error is not in the list: pgx sends COMMIT with the transaction's context,
// so a cancellation can interrupt a COMMIT that the server applies.
func commitRolledBack(err error) bool {
	return dbretry.Retryable(err) || errors.Is(err, sql.ErrTxDone)
}

// sqliteBusy reports a SQLite "database is locked" / "database table is locked" error.
func sqliteBusy(err error) bool {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked
}

func (c *Committer[S]) abort(tx *gorm.DB, reason string, units int, err error) {
	c.logger.WarnContext(c.ctx, "group commit transaction aborted; running the batch ungrouped",
		slog.String("reason", reason), slog.Int("units", units), logging.Error(err))
	if rbErr := tx.Rollback().Error; rbErr != nil {
		c.logger.DebugContext(c.ctx, "rollback of aborted group commit transaction failed", logging.Error(rbErr))
	}
}

// runUnit runs one request's work with an uncancelable copy of the caller's context, recovering a
// panic so it can be re-raised in the caller's goroutine.
func runUnit[S any](r *request[S], scope S) (panicked bool, panicVal any, err error) {
	defer func() {
		if p := recover(); p != nil {
			panicked, panicVal = true, p
		}
	}()

	return false, nil, r.work(context.WithoutCancel(r.ctx), scope)
}
