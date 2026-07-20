package circuitbreaker_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/circuitbreaker"
)

// fakeClock is a deterministic, mutex-guarded clock for tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

const probeInterval = 10 * time.Second

func newBreaker(t *testing.T, threshold uint) (*circuitbreaker.CircuitBreaker, *fakeClock) {
	t.Helper()
	clock := newFakeClock()
	cb := circuitbreaker.New(nil, circuitbreaker.Config{
		FailureThreshold: threshold,
		ProbeInterval:    probeInterval,
		Clock:            clock.Now,
	})
	return cb, clock
}

func TestStaysClosedBelowThreshold(t *testing.T) {
	// given:
	cb, _ := newBreaker(t, 3)

	// when:
	cb.RecordFailure()
	cb.RecordFailure()

	// then:
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestOpensAtThreshold(t *testing.T) {
	tests := map[string]struct {
		threshold uint
		failures  int
	}{
		"threshold 1 opens after 1 failure":  {threshold: 1, failures: 1},
		"threshold 3 opens after 3 failures": {threshold: 3, failures: 3},
		"threshold 0 is treated as 1":        {threshold: 0, failures: 1},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given:
			cb, _ := newBreaker(t, test.threshold)

			// when:
			for i := 0; i < test.failures; i++ {
				cb.RecordFailure()
			}

			// then:
			assert.Equal(t, circuitbreaker.StateOpen, cb.State())
			assert.False(t, cb.Allow())
		})
	}
}

func TestSuccessBetweenFailuresResetsCount(t *testing.T) {
	// given:
	cb, _ := newBreaker(t, 3)

	// when:
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()

	// then:
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestOpenAllowsSingleTrialPerProbeInterval(t *testing.T) {
	// given:
	cb, clock := newBreaker(t, 1)
	cb.RecordFailure()
	require.Equal(t, circuitbreaker.StateOpen, cb.State())
	require.False(t, cb.Allow())

	// when: the probe interval elapses
	clock.Advance(probeInterval)

	// then: exactly one trial is allowed (half-open)
	assert.True(t, cb.Allow())
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())
	assert.False(t, cb.Allow())
	assert.False(t, cb.Allow())

	// when: another probe interval elapses without a recorded result
	clock.Advance(probeInterval)

	// then: one more trial is allowed
	assert.True(t, cb.Allow())
	assert.False(t, cb.Allow())
}

func TestOpenDisallowsBeforeProbeInterval(t *testing.T) {
	// given:
	cb, clock := newBreaker(t, 1)
	cb.RecordFailure()

	// when: not quite a full probe interval has elapsed
	clock.Advance(probeInterval - time.Millisecond)

	// then:
	assert.False(t, cb.Allow())
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())
}

func TestHalfOpenSuccessClosesCircuit(t *testing.T) {
	// given: a half-open circuit
	cb, clock := newBreaker(t, 2)
	cb.RecordFailure()
	cb.RecordFailure()
	clock.Advance(probeInterval)
	require.True(t, cb.Allow())
	require.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	// when:
	cb.RecordSuccess()

	// then:
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	assert.True(t, cb.Allow())

	// and: failure count was reset - a single failure must not reopen (threshold 2)
	cb.RecordFailure()
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
}

func TestHalfOpenFailureReopensAndRestartsWindow(t *testing.T) {
	// given: a half-open circuit
	cb, clock := newBreaker(t, 1)
	cb.RecordFailure()
	clock.Advance(probeInterval)
	require.True(t, cb.Allow())
	require.Equal(t, circuitbreaker.StateHalfOpen, cb.State())

	// when: the trial fails
	cb.RecordFailure()

	// then: circuit reopens
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())
	assert.False(t, cb.Allow())

	// and: the trial window restarts from the failure moment
	clock.Advance(probeInterval - time.Millisecond)
	assert.False(t, cb.Allow())

	clock.Advance(time.Millisecond)
	assert.True(t, cb.Allow())
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())
}

func TestZeroProbeIntervalStillRateLimitsTrials(t *testing.T) {
	// given: a breaker configured with a zero ProbeInterval (defaulted to 1s by New)
	clock := newFakeClock()
	cb := circuitbreaker.New(nil, circuitbreaker.Config{
		FailureThreshold: 1,
		ProbeInterval:    0,
		Clock:            clock.Now,
	})

	// when: the threshold is reached
	cb.RecordFailure()
	require.Equal(t, circuitbreaker.StateOpen, cb.State())

	// then: the open breaker does not admit every call
	assert.False(t, cb.Allow())
	assert.False(t, cb.Allow())

	// and: after the defaulted 1s interval, exactly one trial is allowed
	clock.Advance(time.Second)
	assert.True(t, cb.Allow())
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())
	assert.False(t, cb.Allow())
}

func TestConcurrentAllowAdmitsExactlyOneTrial(t *testing.T) {
	// given: an open breaker whose probe interval has elapsed
	cb, clock := newBreaker(t, 1)
	cb.RecordFailure()
	require.Equal(t, circuitbreaker.StateOpen, cb.State())
	clock.Advance(probeInterval)

	// when: many goroutines race on Allow
	const goroutines = 50
	var allowed atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if cb.Allow() {
				allowed.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	// then: exactly one trial is admitted
	assert.Equal(t, int32(1), allowed.Load())
	assert.Equal(t, circuitbreaker.StateHalfOpen, cb.State())
}

func TestRecordSuccessFromClosedKeepsClosed(t *testing.T) {
	// given:
	cb, _ := newBreaker(t, 3)

	// when:
	cb.RecordSuccess()

	// then:
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestStartHealthProbeClosesCircuitWhenProbeSucceeds(t *testing.T) {
	// given: a probe that fails twice before succeeding
	var mu sync.Mutex
	calls := 0
	probe := func(_ context.Context) bool {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return calls > 2
	}

	cb := circuitbreaker.New(nil, circuitbreaker.Config{
		FailureThreshold: 1,
		ProbeInterval:    10 * time.Millisecond,
		Probe:            probe,
	})
	cb.RecordFailure()
	require.Equal(t, circuitbreaker.StateOpen, cb.State())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		cb.StartHealthProbe(ctx)
	}()

	// then: the circuit eventually closes
	require.Eventually(t, func() bool {
		return cb.State() == circuitbreaker.StateClosed
	}, time.Second, time.Millisecond)

	mu.Lock()
	assert.GreaterOrEqual(t, calls, 3)
	mu.Unlock()

	// when: the context is cancelled
	cancel()

	// then: StartHealthProbe returns
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("StartHealthProbe did not return after context cancellation")
	}
}

func TestStartHealthProbeNilProbeReturnsImmediately(t *testing.T) {
	// given: a breaker without a probe
	cb, _ := newBreaker(t, 1)
	cb.RecordFailure()

	done := make(chan struct{})
	go func() {
		defer close(done)
		cb.StartHealthProbe(context.Background())
	}()

	// then: it returns immediately even though the context is never cancelled
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("StartHealthProbe with nil Probe did not return immediately")
	}
}
