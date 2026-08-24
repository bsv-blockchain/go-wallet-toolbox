package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/service"
)

const overflowWarnMsg = "delayed broadcast queue is full; transactions deferred to the send_waiting cron"

func newOverflowProcess(t *testing.T, queueSize int) (*process, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	return &process{
		logger: logger,
		backgroundBroadcaster: service.NewBackgroundBroadcaster(
			t.Context(), logger, nil, nil, service.Sizing{Workers: 1, ChannelSize: queueSize},
		),
	}, &buf
}

// overflowWarning is the shape the warning is asserted on. Decoding into it rather
// than a map keeps the counters integral, which is what they are.
type overflowWarning struct {
	Msg                      string `json:"msg"`
	DeferredSinceLastWarning uint64 `json:"deferredSinceLastWarning"`
	DeferredTotal            uint64 `json:"deferredTotal"`
	QueueDepth               int    `json:"queueDepth"`
	QueueCapacity            int    `json:"queueCapacity"`
}

func overflowWarnings(t *testing.T, buf *bytes.Buffer) []overflowWarning {
	t.Helper()

	var out []overflowWarning
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec overflowWarning
		require.NoError(t, json.Unmarshal([]byte(line), &rec))
		if rec.Msg == overflowWarnMsg {
			out = append(out, rec)
		}
	}
	return out
}

// TestRecordBroadcasterOverflow_ReportsTheBurstAsOneAccurateWarning is the point of
// the whole change: the incident rejected 197 transactions inside two seconds, and
// logged none of it above debug level. One line per rejection would bury the signal,
// and a line reporting only the first would understate it by two orders of magnitude.
func TestRecordBroadcasterOverflow_ReportsTheBurstAsOneAccurateWarning(t *testing.T) {
	p, buf := newOverflowProcess(t, 1000)

	const rejections = 197
	for range rejections {
		p.recordBroadcasterOverflow(context.Background(), 1)
	}

	// Nothing is emitted mid-burst: the first rejection arms the window rather than
	// logging a count of one.
	require.Empty(t, overflowWarnings(t, buf))

	// The sweep - which is about to pick these very transactions up - flushes it.
	p.flushBroadcasterOverflow(context.Background())

	warnings := overflowWarnings(t, buf)
	require.Len(t, warnings, 1, "a burst must collapse into one warning")

	assert.Equal(t, uint64(rejections), warnings[0].DeferredSinceLastWarning)
	assert.Equal(t, uint64(rejections), warnings[0].DeferredTotal)
	assert.Equal(t, 1000, warnings[0].QueueCapacity)
	assert.NotContains(t, buf.String(), "txIDs", "the warning must not carry per-transaction payload")
	assert.Equal(t, uint64(rejections), p.overflowTotal.Load())
}

// TestFlushBroadcasterOverflow_IsANoopWithNothingPending keeps the sweep quiet on the
// overwhelmingly common path where the queue never overflowed.
func TestFlushBroadcasterOverflow_IsANoopWithNothingPending(t *testing.T) {
	p, buf := newOverflowProcess(t, 10)

	p.flushBroadcasterOverflow(context.Background())
	p.recordBroadcasterOverflow(context.Background(), 4)
	p.flushBroadcasterOverflow(context.Background())
	p.flushBroadcasterOverflow(context.Background())

	assert.Len(t, overflowWarnings(t, buf), 1, "only the flush that had something to report may log")
}

// TestRecordBroadcasterOverflow_WarnsAgainInTheNextWindow makes sure throttling delays
// the warning rather than silencing it while an overflow is ongoing.
func TestRecordBroadcasterOverflow_WarnsAgainInTheNextWindow(t *testing.T) {
	p, buf := newOverflowProcess(t, 10)

	p.recordBroadcasterOverflow(context.Background(), 5) // arms the window
	require.Empty(t, overflowWarnings(t, buf))

	// Age the window instead of sleeping through it.
	p.overflowLoggedAt.Store(time.Now().Add(-2 * broadcasterOverflowLogInterval).UnixNano())
	p.recordBroadcasterOverflow(context.Background(), 3)

	p.overflowLoggedAt.Store(time.Now().Add(-2 * broadcasterOverflowLogInterval).UnixNano())
	p.recordBroadcasterOverflow(context.Background(), 2)

	warnings := overflowWarnings(t, buf)
	require.Len(t, warnings, 2)

	assert.Equal(t, uint64(8), warnings[0].DeferredSinceLastWarning, "the first line carries everything accrued so far")
	assert.Equal(t, uint64(2), warnings[1].DeferredSinceLastWarning, "each later window reports only its own rejections")
	assert.Equal(t, uint64(10), warnings[1].DeferredTotal, "the running total spans windows")
}

func TestRecordBroadcasterOverflow_IgnoresEmptyBatches(t *testing.T) {
	p, buf := newOverflowProcess(t, 10)

	p.recordBroadcasterOverflow(context.Background(), 0)

	assert.Empty(t, overflowWarnings(t, buf))
	assert.Zero(t, p.overflowTotal.Load())
}
