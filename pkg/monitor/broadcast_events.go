package monitor

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// LastEventIDKey is the storage key used to persist the SSE replay cursor across restarts.
const LastEventIDKey = "arcade_sse_last_event_id"

// broadcastEventsReconnectBackoff bounds the outer reconnect loop when a streamer
// returns without the context being canceled. The production Arcade SSE client
// reconnects internally and only returns on cancel; this sleep is a safety net
// for alternate streamers (and for future regressions) so a short-lived return
// cannot hot-loop the CPU.
const broadcastEventsReconnectBackoff = time.Second

// Status-apply pipeline sizing. Events for distinct txids are independent, and
// applying them one-at-a-time (plus one cursor write per event) caps mined
// status/BUMP application at a handful per second — far below what a
// high-throughput stream produces, so fuel never matures and unproven backlog
// grows without bound. A small worker pool with per-batch cursor persistence
// keeps up while preserving replay safety (events are idempotent; the cursor
// only advances to the last event of a fully applied batch).
const (
	broadcastApplyWorkers   = 8
	broadcastApplyBatchMax  = 64
	broadcastApplyQueueSize = 1024

	// broadcastApplyDeadlockAttempts bounds the retry of a deadlocked apply.
	broadcastApplyDeadlockAttempts = 3
)

// BroadcastEventStreamer is implemented by services.WalletServices.
type BroadcastEventStreamer interface {
	BroadcastStatusEvents(ctx context.Context, lastEventID string, onEvent func(wdk.BroadcastStatusEvent) error) error
}

// handleBroadcastEvents consumes the Arcade SSE stream and persists a replay
// cursor after every event so the stream can be resumed after a restart.
// Preferred proof/status path when Arcade is enabled: events push mined status
// and merkle paths so check_for_proofs / MerklePath polling are only a fallback
// if the stream dies or lags. It must be started as a goroutine from Daemon.Start.
func (d *Daemon) handleBroadcastEvents(ctx context.Context, streamer BroadcastEventStreamer) {
	d.logger.InfoContext(ctx, "Starting broadcast event handler")

	// Load the last persisted cursor so we can resume the stream from where we
	// left off.
	id, _, err := d.storage.GetKeyValue(ctx, LastEventIDKey)
	if err != nil {
		d.logger.WarnContext(ctx, "Failed to load SSE replay cursor, starting from beginning", slog.Any("error", err))
	}
	// lastEventID guards the reconnect cursor: the streamer goroutine reads it
	// on reconnect while the applier goroutine advances it after each batch.
	var cursorMu sync.Mutex
	lastEventID := string(id)
	readCursor := func() string {
		cursorMu.Lock()
		defer cursorMu.Unlock()
		return lastEventID
	}

	// The stream callback only enqueues; a separate applier drains the queue in
	// batches and applies events with a bounded worker pool. This pipelines SSE
	// delivery with DB work and amortizes the cursor write over a whole batch
	// instead of one write per event.
	events := make(chan wdk.BroadcastStatusEvent, broadcastApplyQueueSize)
	onEvent := func(ev wdk.BroadcastStatusEvent) error {
		select {
		case events <- ev:
		case <-ctx.Done():
		}
		// Always return nil — we must not wedge the stream on a single bad event.
		return nil
	}

	applierDone := make(chan struct{})
	go func() {
		defer close(applierDone)
		for {
			// Block for the first event of a batch, then drain what is ready.
			var batch []wdk.BroadcastStatusEvent
			select {
			case <-ctx.Done():
				return
			case ev := <-events:
				batch = append(batch, ev)
			}
		drain:
			for len(batch) < broadcastApplyBatchMax {
				select {
				case ev := <-events:
					batch = append(batch, ev)
				default:
					break drain
				}
			}

			d.applyBroadcastEventBatch(ctx, batch, &cursorMu, &lastEventID)
		}
	}()

	// Reconnect loop: if the stream terminates unexpectedly (non-context error)
	// we restart it from the most-recently persisted cursor so no events are
	// missed. Always sleep between attempts so a streamer that returns immediately
	// cannot spin the CPU (production Arcade StreamEvents only returns on cancel
	// and reconnects internally with its own backoff).
	for ctx.Err() == nil {
		streamErr := streamer.BroadcastStatusEvents(ctx, readCursor(), onEvent)
		if ctx.Err() != nil {
			break
		}
		if streamErr != nil {
			d.logger.ErrorContext(ctx, "Broadcast event stream terminated unexpectedly, will retry", slog.Any("error", streamErr))
		} else {
			d.logger.WarnContext(ctx, "Broadcast event stream returned without error, will retry")
		}
		select {
		case <-ctx.Done():
		case <-time.After(broadcastEventsReconnectBackoff):
		}
	}

	<-applierDone
	d.logger.InfoContext(ctx, "Broadcast event handler stopped")
}

// applyBroadcastEvent applies one status event, retrying the deadlocks that
// parallel appliers can form with each other or with the background broadcaster
// (Postgres SQLSTATE 40P01) — the victim is safe to retry immediately. Failures
// are logged and swallowed: replaying an event is safe, and the polling tasks
// are the safety net.
func (d *Daemon) applyBroadcastEvent(ctx context.Context, ev wdk.BroadcastStatusEvent) {
	var results []wdk.TxSynchronizedStatus
	var err error
	for range broadcastApplyDeadlockAttempts {
		results, err = d.storage.ProcessExternalTxStatusUpdate(ctx, ev)
		if err == nil || !strings.Contains(err.Error(), "deadlock detected") {
			break
		}
	}
	if err != nil {
		d.logger.ErrorContext(ctx, "ProcessExternalTxStatusUpdate failed",
			slog.String("txID", ev.TxID),
			slog.String("eventID", ev.EventID),
			slog.Any("error", err),
		)
		return
	}
	d.sendProvenEvents(results)
}

// applyBroadcastEventBatch applies a batch of SSE events with bounded
// concurrency, forwards proven results, and persists the replay cursor once —
// to the last ID-carrying event of the batch — after every event in the batch
// has been attempted. Failed events are logged and skipped (replaying them is
// safe; the polling tasks are the safety net), matching the previous
// one-event-at-a-time semantics.
func (d *Daemon) applyBroadcastEventBatch(ctx context.Context, batch []wdk.BroadcastStatusEvent, cursorMu *sync.Mutex, lastEventID *string) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(broadcastApplyWorkers)
	for _, ev := range batch {
		g.Go(func() error {
			d.applyBroadcastEvent(gctx, ev)
			return nil // a failed event never fails the batch; the cursor still advances
		})
	}
	_ = g.Wait()

	// Persist the cursor once per batch, using the newest event that actually
	// carries an ID: an empty EventID must never overwrite a valid persisted
	// cursor with "" (a restart would then resume with no Last-Event-ID and
	// skip every event in the gap — replaying events is safe, skipping is not).
	cursorID := ""
	for i := len(batch) - 1; i >= 0; i-- {
		if batch[i].EventID != "" {
			cursorID = batch[i].EventID
			break
		}
	}
	if cursorID == "" {
		d.logger.WarnContext(ctx, "Broadcast event batch carried no event IDs, replay cursor not advanced",
			slog.Int("batchSize", len(batch)))
		return
	}
	if persistErr := d.storage.SetKeyValue(ctx, LastEventIDKey, []byte(cursorID)); persistErr != nil {
		// Keep the old in-memory cursor on persist failure so reconnects resume
		// from the last durably persisted position instead of a position the
		// durable cursor never reached.
		d.logger.ErrorContext(ctx, "Failed to persist SSE replay cursor",
			slog.String("eventID", cursorID),
			slog.Any("error", persistErr),
		)
		return
	}
	cursorMu.Lock()
	*lastEventID = cursorID
	cursorMu.Unlock()
}
