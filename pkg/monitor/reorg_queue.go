package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// reorgQueueCheckInterval caps how often the delayed-mode handler looks for aged headers;
// a shorter configured age is used instead so retries are not late by a whole minute.
const reorgQueueCheckInterval = time.Minute

// deactivatedHeader is an orphaned block hash waiting to be re-proved.
type deactivatedHeader struct {
	hash  string
	when  time.Time
	tries uint
}

// reorgQueue is the in-memory, de-duplicated, bounded FIFO of orphaned block hashes used in
// delayed reorg mode (TypeScript Monitor.deactivatedHeaders).
//
// ponytail: in-memory like TS, so queued hashes are lost on restart; persist to a table if that matters.
type reorgQueue struct {
	mu      sync.Mutex
	items   []deactivatedHeader
	hashes  map[string]struct{}
	maxSize int
}

func newReorgQueue(maxSize int) *reorgQueue {
	return &reorgQueue{hashes: make(map[string]struct{}), maxSize: maxSize}
}

// enqueue adds item unless its hash is already queued. When the queue is full the oldest
// entry is evicted; that is reported by returning it.
func (q *reorgQueue) enqueue(item deactivatedHeader) (evicted *deactivatedHeader) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, ok := q.hashes[item.hash]; ok {
		return nil
	}

	if len(q.items) >= q.maxSize {
		oldest := q.items[0]
		q.items = q.items[1:]
		delete(q.hashes, oldest.hash)
		evicted = &oldest
	}

	q.items = append(q.items, item)
	q.hashes[item.hash] = struct{}{}
	return evicted
}

// popAged removes and returns the entries queued at or before cutoff. Entries are always
// enqueued with the current time, so the slice is ordered and we can stop at the first young one.
func (q *reorgQueue) popAged(cutoff time.Time) []deactivatedHeader {
	q.mu.Lock()
	defer q.mu.Unlock()

	n := 0
	for n < len(q.items) && !q.items[n].when.After(cutoff) {
		delete(q.hashes, q.items[n].hash)
		n++
	}

	aged := q.items[:n:n]
	q.items = q.items[n:]
	return aged
}

func (d *Daemon) enqueueDeactivatedHeader(ctx context.Context, item deactivatedHeader) {
	if evicted := d.reorgQueue.enqueue(item); evicted != nil {
		d.logger.ErrorContext(
			ctx, "Reorg queue reached capacity; evicted its oldest header",
			slog.String("evicted_hash", evicted.hash),
			slog.Int("capacity", d.reorgQueue.maxSize),
		)
	}
}

// processReorgQueue periodically re-proves the transactions of orphaned blocks that have
// waited at least the configured age (TypeScript TaskReorg). Headers whose transactions are
// still unchanged or unavailable are re-queued for another age window until MaxRetries is
// reached; after that the original proofs are retained.
func (d *Daemon) processReorgQueue(ctx context.Context) {
	d.logger.InfoContext(ctx, "Starting delayed reorg processor", slog.Duration("age", d.reorgConfig.Age()))

	ticker := time.NewTicker(d.reorgQueueCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.InfoContext(ctx, "delayed reorg processor stopped")
			return
		case <-ticker.C:
			d.reproveAgedHeaders(ctx)
		}
	}
}

func (d *Daemon) reproveAgedHeaders(ctx context.Context) {
	for _, header := range d.reorgQueue.popAged(time.Now().Add(-d.reorgConfig.Age())) {
		if ctx.Err() != nil {
			return
		}

		logger := d.logger.With(slog.String("orphaned_hash", header.hash), slog.Uint64("tries", uint64(header.tries)))

		settled := false
		result, err := d.storage.ReproveHeader(ctx, header.hash)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to reprove orphaned block", logging.Error(err))
		} else {
			d.sendProvenEvents(result.Updated)
			settled = len(result.Unchanged) == 0 && len(result.Unavailable) == 0
		}

		if settled {
			continue
		}

		if header.tries+1 >= d.reorgConfig.MaxRetries {
			logger.WarnContext(ctx, "Maximum reorg retries exceeded; original proofs retained",
				slog.Uint64("max_retries", uint64(d.reorgConfig.MaxRetries)))
			continue
		}

		logger.InfoContext(ctx, "Reorg reprove incomplete; retrying after another age window")
		d.enqueueDeactivatedHeader(ctx, deactivatedHeader{hash: header.hash, when: time.Now(), tries: header.tries + 1})
	}
}
