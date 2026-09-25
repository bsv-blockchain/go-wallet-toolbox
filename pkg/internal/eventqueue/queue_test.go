package eventqueue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/eventqueue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

func TestPublishNeverBlocksAndDeliversEverythingInOrder(t *testing.T) {
	// given: a subscriber channel much smaller than the burst, and nobody reading yet
	const events = 100_000
	out := make(chan int, 10)
	q := eventqueue.New("test", out, logging.NewTestLogger(t))

	// when: the whole burst is published
	start := time.Now()
	for i := range events {
		q.Publish(i)
	}

	// then: publishing did not wait for the subscriber
	assert.Less(t, time.Since(start), 5*time.Second)

	// and: once the subscriber reads, every event arrives exactly once, in order
	for want := range events {
		select {
		case got := <-out:
			require.Equal(t, want, got)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for event %d", want)
		}
	}

	assert.Zero(t, q.Close(t.Context()))
}

func TestStatsReportBacklog(t *testing.T) {
	// given: a subscriber that does not read
	out := make(chan int, 1)
	q := eventqueue.New("test", out, logging.NewTestLogger(t))
	defer q.Discard()

	// when:
	for i := range 11 {
		q.Publish(i)
	}

	// then: one event sits in the channel, the rest are buffered
	waitForDepth(t, q, 10)
	_, oldest := q.Stats()
	assert.Positive(t, oldest)
}

func TestCloseDrainsBacklog(t *testing.T) {
	// given: a buffered backlog
	out := make(chan int, 1)
	q := eventqueue.New("test", out, logging.NewTestLogger(t))
	for i := range 100 {
		q.Publish(i)
	}

	var got []int
	var wg sync.WaitGroup
	wg.Go(func() {
		for v := range out {
			got = append(got, v)
		}
	})

	// when: the owner closes while the subscriber is still reading
	undelivered := q.Close(t.Context())
	close(out) // safe: the forwarder has exited
	wg.Wait()

	// then:
	assert.Zero(t, undelivered)
	require.Len(t, got, 100)
	for i, v := range got {
		assert.Equal(t, i, v)
	}
}

func TestCloseGivesUpAtDeadlineAndStopsSending(t *testing.T) {
	// given: a subscriber that never reads
	out := make(chan int, 1)
	q := eventqueue.New("test", out, logging.NewTestLogger(t))
	for i := range 10 {
		q.Publish(i)
	}
	waitForDepth(t, q, 9) // one event moved into the channel buffer

	// when:
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	undelivered := q.Close(ctx)

	// then: one event reached the channel buffer, the rest are reported
	assert.Equal(t, 9, undelivered)

	// and: nothing sends anymore, so closing the channel cannot panic
	assert.NotPanics(t, func() { close(out) })
}

func TestDiscardDropsBacklogAndStopsSending(t *testing.T) {
	// given:
	out := make(chan int, 1)
	q := eventqueue.New("test", out, logging.NewTestLogger(t))
	for i := range 10 {
		q.Publish(i)
	}
	waitForDepth(t, q, 9) // one event moved into the channel buffer

	// when:
	dropped := q.Discard()

	// then:
	assert.Equal(t, 9, dropped)
	assert.NotPanics(t, func() { close(out) })
}

func TestPublishAfterCloseDoesNotPanic(t *testing.T) {
	// given:
	out := make(chan int, 1)
	q := eventqueue.New("test", out, logging.NewTestLogger(t))
	q.Close(t.Context())
	close(out)

	// expect:
	assert.NotPanics(t, func() { q.Publish(1) })
	depth, _ := q.Stats()
	assert.Zero(t, depth)
}

func TestNilChannelYieldsNoopQueue(t *testing.T) {
	// given:
	q := eventqueue.New[int]("test", nil, logging.NewTestLogger(t))

	// expect:
	assert.Nil(t, q)
	assert.NotPanics(t, func() {
		q.Publish(1)
		assert.Zero(t, q.Close(t.Context()))
		assert.Zero(t, q.Discard())
	})
}

func TestConcurrentProducersLoseNothing(t *testing.T) {
	// given:
	const producers, perProducer = 8, 5_000
	out := make(chan int, 1)
	q := eventqueue.New("test", out, logging.NewTestLogger(t))

	// when: several producers publish while a slow subscriber reads
	var wg sync.WaitGroup
	for range producers {
		wg.Go(func() {
			for i := range perProducer {
				q.Publish(i)
			}
		})
	}

	received := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range out {
			received++
		}
	}()

	wg.Wait()
	require.Zero(t, q.Close(t.Context()))
	close(out)
	<-done

	// then:
	assert.Equal(t, producers*perProducer, received)
}

func waitForDepth(t *testing.T, q *eventqueue.Queue[int], depth int) {
	t.Helper()
	require.Eventually(t, func() bool {
		got, _ := q.Stats()
		return got == depth
	}, time.Second, time.Millisecond)
}
