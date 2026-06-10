package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arcade"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/circuitbreaker"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const testBroadcastTxID = "test-tx-id"

// fakeTarget is a scripted broadcastTarget that records every call.
type fakeTarget struct {
	name      string
	calls     int
	responses []func() (*wdk.PostedTxID, error)
}

func (f *fakeTarget) post(_ context.Context, _ string, _ []byte, txID string) (*wdk.PostedTxID, error) {
	idx := f.calls
	f.calls++
	if idx >= len(f.responses) {
		panic(fmt.Sprintf("unexpected call %d to target %s", idx+1, f.name))
	}
	return f.responses[idx]()
}

func (f *fakeTarget) target() broadcastTarget {
	return broadcastTarget{name: f.name, post: f.post}
}

// foldedTarget wires the fake the way newBroadcastRouter wires the real
// failover services (ARC, WoC, Bitails): the raw client folds transport
// failures into the result and the target unwraps them via
// resultAsTransportOutcome.
func (f *fakeTarget) foldedTarget() broadcastTarget {
	return broadcastTarget{
		name: f.name,
		post: func(ctx context.Context, efHex string, rawTx []byte, txID string) (*wdk.PostedTxID, error) {
			return resultAsTransportOutcome(f.post(ctx, efHex, rawTx, txID))
		},
	}
}

func successResponse(txID string) func() (*wdk.PostedTxID, error) {
	return func() (*wdk.PostedTxID, error) {
		return &wdk.PostedTxID{TxID: txID, Result: wdk.PostedTxIDResultSuccess}, nil
	}
}

func validationErrorResponse(txID string) func() (*wdk.PostedTxID, error) {
	return func() (*wdk.PostedTxID, error) {
		return &wdk.PostedTxID{TxID: txID, Result: wdk.PostedTxIDResultError, Error: errors.New("tx rejected")}, nil
	}
}

func transportErrorResponse() func() (*wdk.PostedTxID, error) {
	return func() (*wdk.PostedTxID, error) {
		return nil, errors.New("connection refused")
	}
}

// foldedTransportErrorResponse mimics clients (arc.PostEF, whatsonchain.PostTX,
// bitails.PostTX) that fold a transport failure into the result: err == nil
// with the result's Error field set.
func foldedTransportErrorResponse(txID string) func() (*wdk.PostedTxID, error) {
	return func() (*wdk.PostedTxID, error) {
		return &wdk.PostedTxID{TxID: txID, Result: wdk.PostedTxIDResultError, Error: errors.New("connection refused")}, nil
	}
}

// doubleSpendHintResponse mimics WhatsOnChain's missing-inputs answer: a
// double-spend hint verdict with no result-level error.
func doubleSpendHintResponse(txID string) func() (*wdk.PostedTxID, error) {
	return func() (*wdk.PostedTxID, error) {
		return &wdk.PostedTxID{TxID: txID, Result: wdk.PostedTxIDResultMissingInputs, DoubleSpend: true}, nil
	}
}

func backpressureResponse(retryAfter time.Duration) func() (*wdk.PostedTxID, error) {
	return func() (*wdk.PostedTxID, error) {
		return nil, &arcade.BackpressureError{RetryAfter: retryAfter}
	}
}

type routerFixture struct {
	t         *testing.T
	primary   *fakeTarget
	failovers []*fakeTarget
	breaker   *circuitbreaker.CircuitBreaker
	clock     *time.Time
	router    *broadcastRouter
	sleeps    []time.Duration
}

func newRouterFixture(t *testing.T, failureThreshold uint, probeInterval time.Duration) *routerFixture {
	t.Helper()

	now := time.Now()
	f := &routerFixture{
		t:       t,
		primary: &fakeTarget{name: "Arcade"},
		clock:   &now,
	}
	f.breaker = circuitbreaker.New(logging.NewTestLogger(t), circuitbreaker.Config{
		FailureThreshold: failureThreshold,
		ProbeInterval:    probeInterval,
		Clock:            func() time.Time { return *f.clock },
	})
	f.router = &broadcastRouter{
		logger:              logging.NewTestLogger(t),
		primary:             f.primary.target(),
		breaker:             f.breaker,
		maxBackpressureWait: defaultMaxBackpressureWait,
		sleep: func(_ context.Context, d time.Duration) {
			f.sleeps = append(f.sleeps, d)
		},
	}
	return f
}

func (f *routerFixture) withFailovers(targets ...*fakeTarget) *routerFixture {
	f.failovers = targets
	for _, target := range targets {
		f.router.failovers = append(f.router.failovers, target.target())
	}
	return f
}

func (f *routerFixture) withFoldedFailovers(targets ...*fakeTarget) *routerFixture {
	f.failovers = targets
	for _, target := range targets {
		f.router.failovers = append(f.router.failovers, target.foldedTarget())
	}
	return f
}

func (f *routerFixture) broadcast() []*wdk.PostFromBEEFServiceResult {
	f.t.Helper()
	return f.router.broadcast(f.t.Context(), "deadbeef", []byte{0xde, 0xad, 0xbe, 0xef}, testBroadcastTxID)
}

func (f *routerFixture) advanceClock(d time.Duration) {
	*f.clock = f.clock.Add(d)
}

func TestBroadcastRouterHappyPath(t *testing.T) {
	// given:
	given := newRouterFixture(t, 3, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){successResponse(testBroadcastTxID)}
	failover := &fakeTarget{name: "ARC"}
	given.withFailovers(failover)

	// when:
	results := given.broadcast()

	// then: exactly one result, from the primary only
	require.Len(t, results, 1)
	assert.Equal(t, "Arcade", results[0].Name)
	require.NoError(t, results[0].Error)
	require.NotNil(t, results[0].PostedBEEFResult)
	require.Len(t, results[0].PostedBEEFResult.TxIDResults, 1)
	assert.Equal(t, wdk.PostedTxIDResultSuccess, results[0].PostedBEEFResult.TxIDResults[0].Result)

	// and: no failover was attempted
	assert.Zero(t, failover.calls)
	assert.Equal(t, circuitbreaker.StateClosed, given.breaker.State())
}

func TestBroadcastRouterValidationErrorDoesNotFailOver(t *testing.T) {
	// given: the primary answers with a tx-level rejection (err == nil)
	given := newRouterFixture(t, 1, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){validationErrorResponse(testBroadcastTxID)}
	failover := &fakeTarget{name: "ARC"}
	given.withFailovers(failover)

	// when:
	results := given.broadcast()

	// then: the rejection is returned as the single result - a rejected tx is not a service failure
	require.Len(t, results, 1)
	assert.Equal(t, "Arcade", results[0].Name)
	require.NotNil(t, results[0].PostedBEEFResult)
	assert.Equal(t, wdk.PostedTxIDResultError, results[0].PostedBEEFResult.TxIDResults[0].Result)

	// and: no failover, breaker stays closed
	assert.Zero(t, failover.calls)
	assert.Equal(t, circuitbreaker.StateClosed, given.breaker.State())
}

func TestBroadcastRouterTransportErrorFailsOverInOrder(t *testing.T) {
	// given: primary fails on transport, first failover fails, second succeeds
	given := newRouterFixture(t, 3, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){transportErrorResponse()}
	first := &fakeTarget{name: "ARC", responses: []func() (*wdk.PostedTxID, error){transportErrorResponse()}}
	second := &fakeTarget{name: "ARC-GorillaPool", responses: []func() (*wdk.PostedTxID, error){successResponse(testBroadcastTxID)}}
	third := &fakeTarget{name: "WhatsOnChain"}
	given.withFailovers(first, second, third)

	// when:
	results := given.broadcast()

	// then: results carry the primary error, the first failover error and the winning result
	require.Len(t, results, 3)
	assert.Equal(t, "Arcade", results[0].Name)
	require.Error(t, results[0].Error)
	assert.Equal(t, "ARC", results[1].Name)
	require.Error(t, results[1].Error)
	assert.Equal(t, "ARC-GorillaPool", results[2].Name)
	require.NoError(t, results[2].Error)
	require.NotNil(t, results[2].PostedBEEFResult)

	// and: the chain stopped at the first success
	assert.Equal(t, 1, first.calls)
	assert.Equal(t, 1, second.calls)
	assert.Zero(t, third.calls)
}

func TestBroadcastRouterAllTargetsFail(t *testing.T) {
	// given:
	given := newRouterFixture(t, 3, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){transportErrorResponse()}
	first := &fakeTarget{name: "ARC", responses: []func() (*wdk.PostedTxID, error){transportErrorResponse()}}
	second := &fakeTarget{name: "WhatsOnChain", responses: []func() (*wdk.PostedTxID, error){transportErrorResponse()}}
	given.withFailovers(first, second)

	// when:
	results := given.broadcast()

	// then: every attempted service is reported as an error result
	require.Len(t, results, 3)
	for _, result := range results {
		require.Error(t, result.Error, "expected error result for %s", result.Name)
		assert.Nil(t, result.PostedBEEFResult)
	}
	assert.Equal(t, []string{"Arcade", "ARC", "WhatsOnChain"}, []string{results[0].Name, results[1].Name, results[2].Name})
}

func TestBroadcastRouterBackpressureRetriesPrimary(t *testing.T) {
	// given: primary asks for backpressure once, then succeeds
	given := newRouterFixture(t, 1, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){
		backpressureResponse(7 * time.Second),
		successResponse(testBroadcastTxID),
	}
	failover := &fakeTarget{name: "ARC"}
	given.withFailovers(failover)

	// when:
	results := given.broadcast()

	// then: the Retry-After hint was honored and the retry succeeded
	assert.Equal(t, []time.Duration{7 * time.Second}, given.sleeps)
	require.Len(t, results, 1)
	assert.Equal(t, "Arcade", results[0].Name)
	require.NoError(t, results[0].Error)

	// and: backpressure did not trip the breaker (threshold is 1) and no failover happened
	assert.Equal(t, circuitbreaker.StateClosed, given.breaker.State())
	assert.Zero(t, failover.calls)
	assert.Equal(t, 2, given.primary.calls)
}

func TestBroadcastRouterBackpressureWaitIsBounded(t *testing.T) {
	// given: the server hints a Retry-After far above the configured maximum
	given := newRouterFixture(t, 3, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){
		backpressureResponse(10 * time.Minute),
		successResponse(testBroadcastTxID),
	}

	// when:
	results := given.broadcast()

	// then: the sleep is capped at maxBackpressureWait
	assert.Equal(t, []time.Duration{defaultMaxBackpressureWait}, given.sleeps)
	require.Len(t, results, 1)
	require.NoError(t, results[0].Error)
}

func TestBroadcastRouterSecondBackpressureFailsOver(t *testing.T) {
	// given: primary applies backpressure twice in a row
	given := newRouterFixture(t, 3, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){
		backpressureResponse(2 * time.Second),
		backpressureResponse(2 * time.Second),
	}
	failover := &fakeTarget{name: "ARC", responses: []func() (*wdk.PostedTxID, error){successResponse(testBroadcastTxID)}}
	given.withFailovers(failover)

	// when:
	results := given.broadcast()

	// then: only one sleep (the retry is not slept on again), then failover
	assert.Equal(t, []time.Duration{2 * time.Second}, given.sleeps)
	require.Len(t, results, 2)
	assert.Equal(t, "Arcade", results[0].Name)
	require.Error(t, results[0].Error)
	assert.Equal(t, "ARC", results[1].Name)
	require.NoError(t, results[1].Error)

	// and: the fall-through counted as a single breaker failure (threshold 3 - still closed)
	assert.Equal(t, circuitbreaker.StateClosed, given.breaker.State())
}

func TestBroadcastRouterCancelledDuringBackpressureDoesNotRetryOrTripBreaker(t *testing.T) {
	// given: failure threshold 1 - a single recorded failure would open the circuit
	given := newRouterFixture(t, 1, time.Minute)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	given.primary.responses = []func() (*wdk.PostedTxID, error){backpressureResponse(5 * time.Second)}

	// and: the caller goes away while the router honors the Retry-After hint
	given.router.sleep = func(_ context.Context, d time.Duration) {
		given.sleeps = append(given.sleeps, d)
		cancel()
	}
	failover := &fakeTarget{name: "ARC"}
	given.withFailovers(failover)

	// when:
	results := given.router.broadcast(ctx, "deadbeef", []byte{0xde, 0xad, 0xbe, 0xef}, testBroadcastTxID)

	// then: no retry of the primary, no failover, a single error result for the primary
	assert.Equal(t, []time.Duration{5 * time.Second}, given.sleeps)
	assert.Equal(t, 1, given.primary.calls)
	assert.Zero(t, failover.calls)
	require.Len(t, results, 1)
	assert.Equal(t, "Arcade", results[0].Name)
	require.ErrorIs(t, results[0].Error, context.Canceled)

	// and: the cancellation did not poison the circuit breaker (threshold 1 - still closed)
	assert.Equal(t, circuitbreaker.StateClosed, given.breaker.State())
}

func TestBroadcastRouterPrimaryContextErrorDoesNotTripBreaker(t *testing.T) {
	tests := map[string]struct {
		primaryErr error
	}{
		"primary returns context.Canceled": {
			primaryErr: fmt.Errorf("post: %w", context.Canceled),
		},
		"primary returns context.DeadlineExceeded": {
			primaryErr: fmt.Errorf("post: %w", context.DeadlineExceeded),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given: failure threshold 1 - a single recorded failure would open the circuit
			given := newRouterFixture(t, 1, time.Minute)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			// and: the primary observes the caller going away mid-call
			given.primary.responses = []func() (*wdk.PostedTxID, error){
				func() (*wdk.PostedTxID, error) {
					cancel()
					return nil, test.primaryErr
				},
			}
			failover := &fakeTarget{name: "ARC"}
			given.withFailovers(failover)

			// when:
			results := given.router.broadcast(ctx, "deadbeef", []byte{0xde, 0xad, 0xbe, 0xef}, testBroadcastTxID)

			// then: the error result for the primary is reported, but no failover is
			// attempted (the context is dead, failovers cannot succeed)
			require.Len(t, results, 1)
			assert.Equal(t, "Arcade", results[0].Name)
			require.ErrorIs(t, results[0].Error, test.primaryErr)
			assert.Zero(t, failover.calls)

			// and: the breaker recorded no failure (threshold 1 - still closed)
			assert.Equal(t, circuitbreaker.StateClosed, given.breaker.State())
		})
	}
}

func TestBroadcastRouterPerCallDeadlineWithLiveContextFailsOverWithoutTrippingBreaker(t *testing.T) {
	// given: failure threshold 1 and a primary whose own (per-call) deadline expired
	// while the caller's context is still alive
	given := newRouterFixture(t, 1, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){
		func() (*wdk.PostedTxID, error) {
			return nil, fmt.Errorf("post: %w", context.DeadlineExceeded)
		},
	}
	failover := &fakeTarget{name: "ARC", responses: []func() (*wdk.PostedTxID, error){successResponse(testBroadcastTxID)}}
	given.withFailovers(failover)

	// when:
	results := given.broadcast()

	// then: the failover is consulted and wins
	require.Len(t, results, 2)
	assert.Equal(t, "Arcade", results[0].Name)
	require.ErrorIs(t, results[0].Error, context.DeadlineExceeded)
	assert.Equal(t, "ARC", results[1].Name)
	require.NoError(t, results[1].Error)

	// and: the deadline error did not count against the breaker (threshold 1 - still closed)
	assert.Equal(t, circuitbreaker.StateClosed, given.breaker.State())
}

func TestBroadcastRouterNilResultWithoutErrorIsReportedNotPanicking(t *testing.T) {
	// given: a primary that breaks the broadcastTarget contract (nil result, nil error)
	given := newRouterFixture(t, 3, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){
		func() (*wdk.PostedTxID, error) { return nil, nil },
	}

	// when:
	results := given.broadcast()

	// then: an error result instead of a panic
	require.Len(t, results, 1)
	assert.Equal(t, "Arcade", results[0].Name)
	require.ErrorContains(t, results[0].Error, "service returned no result")
	assert.Nil(t, results[0].PostedBEEFResult)
}

func TestBroadcastRouterOpenCircuitWithoutFailoversReturnsErrorResult(t *testing.T) {
	// given: a breaker tripped by a previous transport failure (threshold 1) and no failovers
	given := newRouterFixture(t, 1, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){transportErrorResponse()}

	// and: the first broadcast trips the circuit
	_ = given.broadcast()
	require.Equal(t, circuitbreaker.StateOpen, given.breaker.State())

	// when: broadcasting again within the probe window
	results := given.broadcast()

	// then: the primary is skipped, but the caller still gets an error result
	// (zero results would make PostFromBEEF look like a silent success)
	require.Len(t, results, 1)
	assert.Equal(t, "Arcade", results[0].Name)
	require.ErrorContains(t, results[0].Error, "circuit open and no failover targets configured")
	assert.Equal(t, 1, given.primary.calls)
}

func TestBroadcastRouterOpenCircuitSkipsPrimary(t *testing.T) {
	// given: a breaker tripped by a previous transport failure (threshold 1)
	given := newRouterFixture(t, 1, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){transportErrorResponse()}
	failover := &fakeTarget{name: "ARC", responses: []func() (*wdk.PostedTxID, error){
		successResponse(testBroadcastTxID),
		successResponse(testBroadcastTxID),
	}}
	given.withFailovers(failover)

	// and: the first broadcast trips the circuit
	_ = given.broadcast()
	require.Equal(t, circuitbreaker.StateOpen, given.breaker.State())

	// when: broadcasting again within the probe window
	results := given.broadcast()

	// then: the primary is skipped entirely - only the failover responds
	require.Len(t, results, 1)
	assert.Equal(t, "ARC", results[0].Name)
	require.NoError(t, results[0].Error)
	assert.Equal(t, 1, given.primary.calls)
}

func TestBroadcastRouterRecoversAfterTrialSuccess(t *testing.T) {
	// given: a tripped breaker and a primary that has recovered
	given := newRouterFixture(t, 1, time.Minute)
	given.primary.responses = []func() (*wdk.PostedTxID, error){
		transportErrorResponse(),
		successResponse(testBroadcastTxID),
		successResponse(testBroadcastTxID),
	}
	failover := &fakeTarget{name: "ARC", responses: []func() (*wdk.PostedTxID, error){successResponse(testBroadcastTxID)}}
	given.withFailovers(failover)

	// and: trip the circuit
	_ = given.broadcast()
	require.Equal(t, circuitbreaker.StateOpen, given.breaker.State())

	// when: the probe interval elapses, the next broadcast is a half-open trial
	given.advanceClock(time.Minute)
	results := given.broadcast()

	// then: the trial succeeded on the primary and closed the circuit
	require.Len(t, results, 1)
	assert.Equal(t, "Arcade", results[0].Name)
	require.NoError(t, results[0].Error)
	assert.Equal(t, circuitbreaker.StateClosed, given.breaker.State())

	// and: subsequent broadcasts go straight to the primary without waiting for the window
	results = given.broadcast()
	require.Len(t, results, 1)
	assert.Equal(t, "Arcade", results[0].Name)
	assert.Equal(t, 1, failover.calls)
}

func TestResultAsTransportOutcome(t *testing.T) {
	transportErr := errors.New("service is unreachable")

	t.Run("go error passes through", func(t *testing.T) {
		posted, err := resultAsTransportOutcome(nil, transportErr)
		require.ErrorIs(t, err, transportErr)
		assert.Nil(t, posted)
	})

	t.Run("result-level error surfaces as error so failover continues", func(t *testing.T) {
		res := &wdk.PostedTxID{TxID: testBroadcastTxID, Result: wdk.PostedTxIDResultError, Error: transportErr}
		posted, err := resultAsTransportOutcome(res, nil)
		require.ErrorIs(t, err, transportErr)
		assert.Nil(t, posted)
	})

	t.Run("double-spend verdict is a final answer, not a failure", func(t *testing.T) {
		res := &wdk.PostedTxID{TxID: testBroadcastTxID, Result: wdk.PostedTxIDResultError, DoubleSpend: true, Error: errors.New("conflict")}
		posted, err := resultAsTransportOutcome(res, nil)
		require.NoError(t, err)
		require.NotNil(t, posted)
		assert.True(t, posted.DoubleSpend)
	})

	t.Run("success passes through", func(t *testing.T) {
		res := &wdk.PostedTxID{TxID: testBroadcastTxID, Result: wdk.PostedTxIDResultSuccess}
		posted, err := resultAsTransportOutcome(res, nil)
		require.NoError(t, err)
		assert.Equal(t, res, posted)
	})
}

func TestBroadcastRouterFoldedFailoverResults(t *testing.T) {
	tests := map[string]struct {
		wocResponse func() (*wdk.PostedTxID, error)
		// expectLastTargetWins is true when the WoC-style answer must NOT stop
		// the chain, so the last target is consulted and wins.
		expectLastTargetWins bool
	}{
		"transport-folded WoC result (err == nil, result.Error set) no longer stops the chain": {
			wocResponse:          foldedTransportErrorResponse(testBroadcastTxID),
			expectLastTargetWins: true,
		},
		"double-spend hint verdict is a final answer and stops the chain": {
			wocResponse:          doubleSpendHintResponse(testBroadcastTxID),
			expectLastTargetWins: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// given: the primary fails on transport and the failovers fold their
			// answers into the result like the real clients do
			given := newRouterFixture(t, 3, time.Minute)
			given.primary.responses = []func() (*wdk.PostedTxID, error){transportErrorResponse()}
			woc := &fakeTarget{name: "WhatsOnChain", responses: []func() (*wdk.PostedTxID, error){test.wocResponse}}
			last := &fakeTarget{name: "Bitails", responses: []func() (*wdk.PostedTxID, error){successResponse(testBroadcastTxID)}}
			given.withFoldedFailovers(woc, last)

			// when:
			results := given.broadcast()

			// then: the primary transport failure is always the first error result
			assert.Equal(t, "Arcade", results[0].Name)
			require.Error(t, results[0].Error)

			if test.expectLastTargetWins {
				// and: the folded WoC failure is reported as an error and the chain went on
				require.Len(t, results, 3)
				assert.Equal(t, "WhatsOnChain", results[1].Name)
				require.Error(t, results[1].Error)
				assert.Nil(t, results[1].PostedBEEFResult)
				assert.Equal(t, "Bitails", results[2].Name)
				require.NoError(t, results[2].Error)
				require.NotNil(t, results[2].PostedBEEFResult)
				assert.Equal(t, 1, last.calls)
			} else {
				// and: the double-spend verdict ends the chain - no spraying to more services
				require.Len(t, results, 2)
				assert.Equal(t, "WhatsOnChain", results[1].Name)
				require.NoError(t, results[1].Error)
				require.NotNil(t, results[1].PostedBEEFResult)
				assert.True(t, results[1].PostedBEEFResult.TxIDResults[0].DoubleSpend)
				assert.Zero(t, last.calls)
			}
		})
	}
}
