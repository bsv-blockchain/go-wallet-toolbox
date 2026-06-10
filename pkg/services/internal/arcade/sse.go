package arcade

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	sseBackoffBase = 1 * time.Second
	sseBackoffMax  = 60 * time.Second

	// readWatchdogTimeout is the default read-liveness watchdog of the SSE stream:
	// Arcade sends keepalive comments every 15s, so 60s without a single line means
	// a dead TCP peer - the connection is dropped and redialed instead of hanging.
	readWatchdogTimeout = 60 * time.Second

	// sseScannerInitialBufferSize is the initial buffer of the SSE line scanner.
	sseScannerInitialBufferSize = 64 * 1024
	// sseScannerMaxBufferSize is the maximum size of a single SSE line (1 MiB).
	sseScannerMaxBufferSize = 1 << 20
)

// StatusEvent is one SSE status frame from GET /events.
type StatusEvent struct {
	// TXInfo is parsed from the "data:" JSON payload of the frame.
	TXInfo

	// EventID is the SSE "id:" field - a nanosecond timestamp, used as Last-Event-ID on reconnect.
	EventID string
}

// StreamEvents connects to {EventsURL}/events?callbackToken=... and invokes onEvent
// sequentially per status event. It auto-reconnects with exponential backoff
// (1s base, 2x factor, 60s cap; reset after a cleanly-ended connection that delivered
// at least one event - a connection killed by a read error keeps backing off, so a
// permanently oversized frame cannot cause a reconnect hot-loop), sending
// Last-Event-ID from the most recently delivered event (or the lastEventID argument
// before any event arrives). A connection on which nothing is read for the watchdog
// timeout is dropped and redialed.
//
// It blocks until ctx is cancelled and then returns ctx.Err().
// An error returned by onEvent is only logged - the event still counts as delivered.
func (s *Service) StreamEvents(ctx context.Context, lastEventID string, onEvent func(ev StatusEvent) error) error {
	backoff := sseBackoffBase

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		delivered, err := s.streamOnce(ctx, &lastEventID, onEvent)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			s.logger.WarnContext(
				ctx, "arcade events stream interrupted, reconnecting",
				slog.String("error", err.Error()),
				slog.Int("deliveredEvents", delivered),
				slog.Duration("backoff", backoff),
			)
		} else {
			s.logger.DebugContext(
				ctx, "arcade events stream closed by server, reconnecting",
				slog.Int("deliveredEvents", delivered),
				slog.Duration("backoff", backoff),
			)
		}

		// reset backoff only after a cleanly-ended connection that delivered events;
		// a connection that died with a read error (e.g. an oversized frame hitting
		// bufio.ErrTooLong on every reconnect) must keep backing off to avoid a hot-loop.
		if delivered > 0 && err == nil {
			backoff = sseBackoffBase
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, sseBackoffMax)
	}
}

// streamOnce performs a single connection to the events endpoint and dispatches
// status events until the stream ends. It returns the number of delivered events.
//
// A per-connection context backs a read-liveness watchdog: when no line is read for
// s.sseReadWatchdogTimeout, the connection (not the outer ctx) is cancelled so the
// stream reconnects instead of hanging forever on a dead TCP peer.
func (s *Service) streamOnce(ctx context.Context, lastEventID *string, onEvent func(ev StatusEvent) error) (delivered int, err error) {
	connCtx, cancelConn := context.WithCancel(ctx)
	defer cancelConn()

	req, err := http.NewRequestWithContext(connCtx, http.MethodGet, s.eventsURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create arcade events request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "go-wallet-toolbox")
	if *lastEventID != "" {
		req.Header.Set("Last-Event-ID", *lastEventID)
	}

	response, err := s.sseClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to arcade events stream: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, sseScannerInitialBufferSize))
		return 0, fmt.Errorf("arcade events stream returned unexpected http status [%d %s]", response.StatusCode, response.Status)
	}

	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, sseScannerInitialBufferSize), sseScannerMaxBufferSize)

	// read-liveness watchdog: cancel this connection when no line arrives in time
	watchdog := time.AfterFunc(s.sseReadWatchdogTimeout, cancelConn)
	defer watchdog.Stop()

	var frame sseFrame
	for scanner.Scan() {
		watchdog.Reset(s.sseReadWatchdogTimeout)
		line := scanner.Text()

		// a blank line marks the end of a frame
		if line == "" {
			if s.dispatchFrame(ctx, frame, lastEventID, onEvent) {
				delivered++
			}
			frame = sseFrame{}
			continue
		}

		// lines starting with ":" are keepalive comments; all others accumulate into the frame
		if !strings.HasPrefix(line, ":") {
			processSSELine(line, &frame)
		}
	}
	// an incomplete frame at the end of the stream is discarded per the SSE spec

	if scanErr := scanner.Err(); scanErr != nil {
		// a watchdog-triggered cancellation only kills this connection: the outer ctx
		// is still alive, so StreamEvents will reconnect instead of returning.
		if connCtx.Err() != nil && ctx.Err() == nil {
			return delivered, fmt.Errorf("arcade events stream stalled (no data for %s): %w", s.sseReadWatchdogTimeout, scanErr)
		}
		return delivered, fmt.Errorf("arcade events stream read failed: %w", scanErr)
	}
	return delivered, nil
}

// sseFrame accumulates the fields of one SSE frame until a blank line dispatches it.
type sseFrame struct {
	id    string
	event string
	data  string
}

// dispatchFrame parses and delivers one accumulated SSE frame.
// It reports whether the event has been delivered to onEvent.
// Frames with an event type other than "status" (or absent) and frames whose
// data does not parse as TXInfo JSON are skipped without killing the stream.
func (s *Service) dispatchFrame(ctx context.Context, frame sseFrame, lastEventID *string, onEvent func(ev StatusEvent) error) bool {
	if frame.data == "" {
		return false
	}
	if frame.event != "" && frame.event != "status" {
		return false
	}

	var info TXInfo
	if err := json.Unmarshal([]byte(frame.data), &info); err != nil {
		s.logger.WarnContext(
			ctx, "skipping malformed arcade status event",
			slog.String("eventID", frame.id),
			slog.String("error", err.Error()),
		)
		return false
	}
	if info.TxID == "" {
		s.logger.WarnContext(ctx, "skipping arcade status event without txid", slog.String("eventID", frame.id))
		return false
	}

	if err := onEvent(StatusEvent{EventID: frame.id, TXInfo: info}); err != nil {
		s.logger.ErrorContext(
			ctx, "arcade status event handler failed",
			slog.String("eventID", frame.id),
			slog.String("txID", info.TxID),
			slog.String("error", err.Error()),
		)
	}

	// Intentionally NOT SSE-spec behavior (the spec advances Last-Event-ID on every
	// frame carrying an id): Last-Event-ID is advanced only for DELIVERED frames, so
	// frames that were read but not delivered are redelivered by the server after a
	// reconnect (at-least-once delivery).
	if frame.id != "" {
		*lastEventID = frame.id
	}
	return true
}

// processSSELine accumulates one SSE line into the current frame.
// Unknown field names are silently ignored per the SSE spec.
func processSSELine(line string, frame *sseFrame) {
	field, value := splitSSELine(line)
	switch field {
	case "id":
		frame.id = value
	case "event":
		frame.event = value
	case "data":
		if frame.data != "" {
			frame.data += "\n"
		}
		frame.data += value
	}
}

// splitSSELine splits one SSE line into its field name and value,
// stripping the single optional space after the colon per the SSE spec.
func splitSSELine(line string) (field, value string) {
	field, value, found := strings.Cut(line, ":")
	if !found {
		// a line with no colon is treated as a field with an empty value per the SSE spec
		return line, ""
	}
	return field, strings.TrimPrefix(value, " ")
}
