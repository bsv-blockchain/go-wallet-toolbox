package arcade_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arcade"
)

const sseTestTimeout = 10 * time.Second

type recordedSSERequest struct {
	lastEventID   string
	callbackToken string
	accept        string
}

func TestStreamEvents(t *testing.T) {
	// given: an SSE server that closes the stream after the first batch of events
	var requestCount atomic.Int32
	requests := make(chan recordedSSERequest, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !assert.True(t, ok, "response writer must support flushing") {
			return
		}

		requests <- recordedSSERequest{
			lastEventID:   r.Header.Get("Last-Event-ID"),
			callbackToken: r.URL.Query().Get("callbackToken"),
			accept:        r.Header.Get("Accept"),
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		switch requestCount.Add(1) {
		case 1:
			// keepalive comments must be ignored
			_, _ = io.WriteString(w, ": keepalive\n\n")
			_, _ = io.WriteString(w, "id: 1\nevent: status\ndata: {\"txid\":\"tx-aaa\",\"txStatus\":\"SEEN_ON_NETWORK\"}\n\n")
			// malformed data frame must be skipped without killing the stream
			_, _ = io.WriteString(w, "id: 1-bad\nevent: status\ndata: {malformed\n\n")
			_, _ = io.WriteString(w, "id: 2\nevent: status\ndata: {\"txid\":\"tx-bbb\",\"txStatus\":\"MINED\"}\n\n")
			flusher.Flush()
			// returning closes the stream and forces the client to reconnect
		default:
			_, _ = io.WriteString(w, "id: 3\ndata: {\"txid\":\"tx-ccc\",\"txStatus\":\"IMMUTABLE\"}\n\n")
			flusher.Flush()
			<-r.Context().Done()
		}
	}))
	defer server.Close()

	// and:
	service := newService(t, defaultConfig(server.URL))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// when:
	events := make(chan arcade.StatusEvent, 10)
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.StreamEvents(ctx, "", func(ev arcade.StatusEvent) error {
			events <- ev
			if ev.TxID == "tx-aaa" {
				// onEvent error must be logged only - the event still counts as delivered
				return errors.New("handler failed on purpose")
			}
			return nil
		})
	}()

	waitForEvent := func() arcade.StatusEvent {
		t.Helper()
		select {
		case ev := <-events:
			return ev
		case <-time.After(sseTestTimeout):
			t.Fatal("timed out waiting for SSE event")
			return arcade.StatusEvent{}
		}
	}

	// then: events delivered in order, malformed frame skipped
	first := waitForEvent()
	assert.Equal(t, "1", first.EventID)
	assert.Equal(t, "tx-aaa", first.TxID)
	assert.Equal(t, arcade.StatusSeenOnNetwork, first.TxStatus)

	second := waitForEvent()
	assert.Equal(t, "2", second.EventID)
	assert.Equal(t, "tx-bbb", second.TxID)
	assert.Equal(t, arcade.StatusMined, second.TxStatus)

	// and: the client reconnects and keeps receiving events
	third := waitForEvent()
	assert.Equal(t, "3", third.EventID)
	assert.Equal(t, "tx-ccc", third.TxID)
	assert.Equal(t, arcade.StatusImmutable, third.TxStatus)

	// and: the first request carries no Last-Event-ID but the callback token
	firstRequest := <-requests
	assert.Empty(t, firstRequest.lastEventID)
	assert.Equal(t, testCallbackToken, firstRequest.callbackToken)
	assert.Equal(t, "text/event-stream", firstRequest.accept)

	// and: the reconnect carries the id of the last delivered event
	secondRequest := <-requests
	assert.Equal(t, "2", secondRequest.lastEventID)
	assert.Equal(t, testCallbackToken, secondRequest.callbackToken)

	// when:
	cancel()

	// then:
	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(sseTestTimeout):
		t.Fatal("StreamEvents did not return after context cancellation")
	}
}

func TestStreamEventsSendsInitialLastEventID(t *testing.T) {
	// given:
	lastEventIDs := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case lastEventIDs <- r.Header.Get("Last-Event-ID"):
		default:
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// and:
	service := newService(t, defaultConfig(server.URL))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// when:
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.StreamEvents(ctx, "42", func(arcade.StatusEvent) error { return nil })
	}()

	// then: before any event is delivered, the lastEventID argument is sent
	select {
	case lastEventID := <-lastEventIDs:
		assert.Equal(t, "42", lastEventID)
	case <-time.After(sseTestTimeout):
		t.Fatal("timed out waiting for SSE request")
	}

	// cleanup:
	cancel()
	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(sseTestTimeout):
		t.Fatal("StreamEvents did not return after context cancellation")
	}
}

func TestStreamEventsWatchdogReconnectsStalledStream(t *testing.T) {
	// given: a server whose first connection delivers one event and then goes silent
	// (no keepalives, no close) - simulating a dead TCP peer
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !assert.True(t, ok, "response writer must support flushing") {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		switch requestCount.Add(1) {
		case 1:
			_, _ = io.WriteString(w, "id: 1\nevent: status\ndata: {\"txid\":\"tx-stalled\",\"txStatus\":\"SEEN_ON_NETWORK\"}\n\n")
			flusher.Flush()
			// go silent without closing - only the client watchdog can break this
			<-r.Context().Done()
		default:
			_, _ = io.WriteString(w, "id: 2\nevent: status\ndata: {\"txid\":\"tx-after-reconnect\",\"txStatus\":\"MINED\"}\n\n")
			flusher.Flush()
			<-r.Context().Done()
		}
	}))
	defer server.Close()

	// and: a service with a tiny watchdog timeout
	service := newService(t, defaultConfig(server.URL))
	service.SetSSEReadWatchdogTimeoutForTests(150 * time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// when:
	events := make(chan arcade.StatusEvent, 10)
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.StreamEvents(ctx, "", func(ev arcade.StatusEvent) error {
			events <- ev
			return nil
		})
	}()

	waitForEvent := func() arcade.StatusEvent {
		t.Helper()
		select {
		case ev := <-events:
			return ev
		case <-time.After(sseTestTimeout):
			t.Fatal("timed out waiting for SSE event")
			return arcade.StatusEvent{}
		}
	}

	// then: the first event arrives on the stalled connection
	first := waitForEvent()
	assert.Equal(t, "tx-stalled", first.TxID)

	// and: the watchdog drops the dead connection and the stream reconnects
	// (StreamEvents must NOT return - the outer ctx is still alive)
	second := waitForEvent()
	assert.Equal(t, "tx-after-reconnect", second.TxID)

	select {
	case err := <-errCh:
		t.Fatalf("StreamEvents returned after watchdog reconnect: %v", err)
	default:
	}

	// when: the outer context is cancelled
	cancel()

	// then: StreamEvents returns with the context error
	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(sseTestTimeout):
		t.Fatal("StreamEvents did not return after context cancellation")
	}
}

func TestStreamEventsContextCancelWhileConnected(t *testing.T) {
	// given: a server that holds the stream open without sending events
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !assert.True(t, ok, "response writer must support flushing") {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	// and:
	service := newService(t, defaultConfig(server.URL))

	ctx, cancel := context.WithCancel(t.Context())

	// when:
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.StreamEvents(ctx, "", func(arcade.StatusEvent) error { return nil })
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	// then:
	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(sseTestTimeout):
		t.Fatal("StreamEvents did not return after context cancellation")
	}
}

// Arcade docs/sse.md: MINED frames carry blockHash, blockHeight, and merklePath
// (BRC-74 BUMP hex) so push-only clients can apply proofs without polling.
func TestStreamEventsMinedFrameCarriesMerklePath(t *testing.T) {
	const (
		eventID     = "1745870512987654321"
		blockHash   = "000000000000000001885e0c6c302cbbacf927e1b5cf7884588973e72f8b1234"
		blockHeight = uint32(870123)
		// Opaque hex; client treats merklePath as a string until storage validates BUMP.
		merklePathHex = "0100cafe"
	)

	// Frame shape from https://github.com/bsv-blockchain/arcade/blob/main/docs/sse.md
	minedFrame := "id: " + eventID + "\n" +
		"event: status\n" +
		`data: {"txid":"` + testTxID + `","txStatus":"MINED","timestamp":"2026-04-28T18:21:52Z",` +
		`"blockHash":"` + blockHash + `","blockHeight":` + "870123" + `,"merklePath":"` + merklePathHex + `"}` +
		"\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !assert.True(t, ok) {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, minedFrame)
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	service := newService(t, defaultConfig(server.URL))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	events := make(chan arcade.StatusEvent, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.StreamEvents(ctx, "", func(ev arcade.StatusEvent) error {
			events <- ev
			return nil
		})
	}()

	select {
	case ev := <-events:
		assert.Equal(t, eventID, ev.EventID)
		assert.Equal(t, testTxID, ev.TxID)
		assert.Equal(t, arcade.StatusMined, ev.TxStatus)
		assert.Equal(t, blockHash, ev.BlockHash)
		assert.Equal(t, blockHeight, ev.BlockHeight)
		assert.Equal(t, merklePathHex, ev.MerklePath)
		assert.Equal(t, "2026-04-28T18:21:52Z", ev.Timestamp)
	case <-time.After(sseTestTimeout):
		t.Fatal("timed out waiting for MINED SSE frame")
	}

	cancel()
	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(sseTestTimeout):
		t.Fatal("StreamEvents did not return after cancel")
	}
}
