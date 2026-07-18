package monitor

import (
	"context"
	"log/slog"
	"time"

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

// BroadcastEventStreamer is implemented by services.WalletServices.
type BroadcastEventStreamer interface {
	BroadcastStatusEvents(ctx context.Context, lastEventID string, onEvent func(wdk.BroadcastStatusEvent) error) error
}

// handleBroadcastEvents consumes the Arcade SSE stream and persists a replay
// cursor after every event so the stream can be resumed after a restart.
// It must be started as a goroutine from Daemon.Start.
func (d *Daemon) handleBroadcastEvents(ctx context.Context, streamer BroadcastEventStreamer) {
	d.logger.InfoContext(ctx, "Starting broadcast event handler")

	// Load the last persisted cursor so we can resume the stream from where we
	// left off.
	id, _, err := d.storage.GetKeyValue(ctx, LastEventIDKey)
	if err != nil {
		d.logger.WarnContext(ctx, "Failed to load SSE replay cursor, starting from beginning", slog.Any("error", err))
	}
	// lastEventID is declared as a variable (not captured by pointer in the
	// closure below) so that reconnect attempts always use the most-recent cursor.
	lastEventID := string(id)

	onEvent := func(ev wdk.BroadcastStatusEvent) error {
		results, storageErr := d.storage.ProcessExternalTxStatusUpdate(ctx, ev)
		if storageErr != nil {
			d.logger.ErrorContext(ctx, "ProcessExternalTxStatusUpdate failed",
				slog.String("txID", ev.TxID),
				slog.String("eventID", ev.EventID),
				slog.Any("error", storageErr),
			)
			// Fall through — still persist the cursor so we do not replay the
			// same event on restart.  The polling tasks act as the safety net.
		}

		// Persist the replay cursor regardless of whether processing succeeded.
		if persistErr := d.storage.SetKeyValue(ctx, LastEventIDKey, []byte(ev.EventID)); persistErr != nil {
			d.logger.ErrorContext(ctx, "Failed to persist SSE replay cursor",
				slog.String("eventID", ev.EventID),
				slog.Any("error", persistErr),
			)
		}

		// Keep lastEventID up-to-date so reconnect attempts resume from the
		// correct position.
		lastEventID = ev.EventID

		if storageErr == nil {
			d.sendProvenEvents(results)
		}

		// Always return nil — we must not wedge the stream on a single bad event.
		return nil
	}

	// Reconnect loop: if the stream terminates unexpectedly (non-context error)
	// we restart it from the most-recently persisted cursor so no events are
	// missed. Always sleep between attempts so a streamer that returns immediately
	// cannot spin the CPU (production Arcade StreamEvents only returns on cancel
	// and reconnects internally with its own backoff).
	for ctx.Err() == nil {
		streamErr := streamer.BroadcastStatusEvents(ctx, lastEventID, onEvent)
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

	d.logger.InfoContext(ctx, "Broadcast event handler stopped")
}
