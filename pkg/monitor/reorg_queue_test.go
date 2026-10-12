package monitor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// reorgStorage stubs the reorg-related storage calls; the testabilities mock cannot be
// imported here (it imports this package).
type reorgStorage struct {
	MonitoredStorage

	HandleReorgCalled   atomic.Int64
	ReproveHeaderCalled atomic.Int64
	ReproveHeaderResult *wdk.ReproveHeaderResult
}

func (s *reorgStorage) HandleReorg(context.Context, []string) error {
	s.HandleReorgCalled.Add(1)
	return nil
}

func (s *reorgStorage) ReproveHeader(context.Context, string) (*wdk.ReproveHeaderResult, error) {
	s.ReproveHeaderCalled.Add(1)
	if s.ReproveHeaderResult != nil {
		return s.ReproveHeaderResult, nil
	}
	return &wdk.ReproveHeaderResult{}, nil
}

func TestReorgQueue_DeduplicatesEvictsAndPopsAged(t *testing.T) {
	now := time.Now()
	q := newReorgQueue(2)

	// dedup: the second enqueue of the same hash is ignored
	assert.Nil(t, q.enqueue(deactivatedHeader{hash: "a", when: now.Add(-3 * time.Minute)}))
	assert.Nil(t, q.enqueue(deactivatedHeader{hash: "a", when: now}))
	assert.Nil(t, q.enqueue(deactivatedHeader{hash: "b", when: now.Add(-2 * time.Minute)}))

	// capacity: the oldest is evicted
	evicted := q.enqueue(deactivatedHeader{hash: "c", when: now})
	require.NotNil(t, evicted)
	assert.Equal(t, "a", evicted.hash)

	// popAged returns only entries at or before the cutoff, in order
	aged := q.popAged(now.Add(-time.Minute))
	require.Len(t, aged, 1)
	assert.Equal(t, "b", aged[0].hash)

	// a popped hash can be queued again
	assert.Nil(t, q.enqueue(deactivatedHeader{hash: "b", when: now}))
	assert.Len(t, q.popAged(now), 2)
	assert.Empty(t, q.popAged(now))
}

func newDelayedDaemon(t *testing.T, storage MonitoredStorage, opts ...DaemonEventOption) *Daemon {
	t.Helper()
	opts = append(opts, WithReorgConfig(defs.Reorg{
		Mode:             defs.ReorgModeDelayed,
		AgeSeconds:       1,
		MaxRetries:       2,
		MaxQueuedHeaders: 10,
	}))
	d, err := NewDaemon(logging.NewTestLogger(t), storage, DefaultDaemonEventOptions(opts...))
	require.NoError(t, err)
	return d
}

func TestDelayedReorg_QueuesEventAndReprovesAfterAge(t *testing.T) {
	// given:
	storage := &reorgStorage{}
	reorgCh := make(chan *chaintracks.ReorgEvent, 1)
	d := newDelayedDaemon(t, storage, WithReorgChannel(reorgCh))
	d.reorgQueueCheckInterval = 10 * time.Millisecond
	require.NoError(t, d.Start(t.Context(), nil))
	t.Cleanup(func() { _ = d.Stop(t.Context()) })

	// when:
	reorgCh <- &chaintracks.ReorgEvent{Depth: 1, OrphanedHashes: []chainhash.Hash{{1}}}

	// then: nothing happens before the age window
	time.Sleep(300 * time.Millisecond)
	assert.Zero(t, storage.ReproveHeaderCalled.Load())

	// and: the header is reproved once aged, and proofs are never invalidated
	require.Eventually(t, func() bool { return storage.ReproveHeaderCalled.Load() == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.Zero(t, storage.HandleReorgCalled.Load())
}

func TestDelayedReorg_RetriesUntilMaxRetriesThenGivesUp(t *testing.T) {
	// given: storage never finds a replacement proof
	storage := &reorgStorage{ReproveHeaderResult: &wdk.ReproveHeaderResult{Unavailable: []string{"tx"}}}
	d := newDelayedDaemon(t, storage)
	d.enqueueDeactivatedHeader(t.Context(), deactivatedHeader{hash: "h", when: time.Now().Add(-time.Hour)})

	// when: first attempt
	d.reproveAgedHeaders(t.Context())

	// then: re-queued for another age window
	assert.EqualValues(t, 1, storage.ReproveHeaderCalled.Load())
	queued := d.reorgQueue.popAged(time.Now())
	require.Len(t, queued, 1)
	assert.EqualValues(t, 1, queued[0].tries)

	// when: second (last) attempt
	d.enqueueDeactivatedHeader(t.Context(), deactivatedHeader{hash: "h", when: time.Now().Add(-time.Hour), tries: 1})
	d.reproveAgedHeaders(t.Context())

	// then: dropped, original proofs retained
	assert.EqualValues(t, 2, storage.ReproveHeaderCalled.Load())
	assert.Empty(t, d.reorgQueue.popAged(time.Now()))
}

func TestDelayedReorg_SettledHeaderIsNotRetried(t *testing.T) {
	// given:
	storage := &reorgStorage{}
	d := newDelayedDaemon(t, storage)
	d.enqueueDeactivatedHeader(t.Context(), deactivatedHeader{hash: "h", when: time.Now().Add(-time.Hour)})

	// when:
	d.reproveAgedHeaders(t.Context())

	// then:
	assert.EqualValues(t, 1, storage.ReproveHeaderCalled.Load())
	assert.Empty(t, d.reorgQueue.popAged(time.Now()))
}

func TestImmediateReorg_InvalidatesRightAway(t *testing.T) {
	// given: default (immediate) mode
	storage := &reorgStorage{}
	reorgCh := make(chan *chaintracks.ReorgEvent, 1)
	d, err := NewDaemon(logging.NewTestLogger(t), storage, DefaultDaemonEventOptions(WithReorgChannel(reorgCh)))
	require.NoError(t, err)
	require.NoError(t, d.Start(t.Context(), nil))
	t.Cleanup(func() { _ = d.Stop(t.Context()) })

	// when:
	reorgCh <- &chaintracks.ReorgEvent{Depth: 1, OrphanedHashes: []chainhash.Hash{{1}}}

	// then:
	require.Eventually(t, func() bool { return storage.HandleReorgCalled.Load() == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.Zero(t, storage.ReproveHeaderCalled.Load())
}

func TestNewDaemon_RejectsInvalidReorgConfig(t *testing.T) {
	// when: delayed mode without a queue capacity
	_, err := NewDaemon(logging.NewTestLogger(t), &reorgStorage{}, DefaultDaemonEventOptions(WithReorgConfig(defs.Reorg{
		Mode:       defs.ReorgModeDelayed,
		AgeSeconds: 1,
		MaxRetries: 1,
	})))

	// then:
	require.Error(t, err)
}
