package eventqueue_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/eventqueue"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

func TestAcquireSharesOneQueuePerChannel(t *testing.T) {
	// given: two owners handed the same subscriber channel
	out := make(chan int, 1)
	logger := logging.NewTestLogger(t)
	first, releaseFirst := eventqueue.Acquire("test", out, logger)
	second, releaseSecond := eventqueue.Acquire("test", out, logger)

	// then: they share one queue, so the channel has a single sender
	require.Same(t, first, second)

	// when: both publish, then the first owner stops
	first.Publish(1)
	second.Publish(2)
	releaseFirst(t.Context())

	// then: the queue is still open for the second owner
	second.Publish(3)
	for want := 1; want <= 3; want++ {
		assert.Equal(t, want, <-out)
	}

	// when: the last owner releases, the queue closes
	releaseSecond(t.Context())
	assert.NotPanics(t, func() { close(out) })
}

func TestAcquireAfterFullReleaseCreatesNewQueue(t *testing.T) {
	// given:
	out := make(chan int, 1)
	first, release := eventqueue.Acquire("test", out, logging.NewTestLogger(t))
	release(t.Context())
	release(t.Context()) // idempotent

	// when:
	second, releaseSecond := eventqueue.Acquire("test", out, logging.NewTestLogger(t))
	defer releaseSecond(t.Context())

	// then:
	assert.NotSame(t, first, second)
	second.Publish(7)
	assert.Equal(t, 7, <-out)
}

func TestAcquireNilChannel(t *testing.T) {
	// given:
	q, release := eventqueue.Acquire[int]("test", nil, logging.NewTestLogger(t))

	// expect:
	assert.Nil(t, q)
	assert.NotPanics(t, func() { release(t.Context()) })
}
