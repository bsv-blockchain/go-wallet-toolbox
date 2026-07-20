// Package circuitbreaker provides a small, deterministic circuit breaker used to
// gate calls to unhealthy external services. The core is purely synchronous,
// mutex-guarded and clock-injected, so its behavior is fully testable without sleeps.
package circuitbreaker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
)

// State represents the current state of a CircuitBreaker.
type State int

// Circuit breaker states.
const (
	// StateClosed means calls flow normally.
	StateClosed State = iota
	// StateOpen means calls are rejected until a trial window elapses.
	StateOpen
	// StateHalfOpen means a single trial call has been allowed through;
	// its result decides whether the circuit closes or reopens.
	StateHalfOpen
)

// String returns a human-readable name of the state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config configures a CircuitBreaker.
type Config struct {
	// FailureThreshold is the number of consecutive failures required to open the circuit.
	// A value of 0 is treated as 1.
	FailureThreshold uint
	// ProbeInterval is the trial/probe cadence while the circuit is open.
	ProbeInterval time.Duration
	// Probe is an optional health probe used by StartHealthProbe.
	// When nil, recovery is time-based (half-open trials) only.
	Probe func(ctx context.Context) bool
	// Clock returns the current time; injectable for tests. When nil, time.Now is used.
	Clock func() time.Time
}

// CircuitBreaker is a deterministic, mutex-guarded circuit breaker.
// All state transitions happen synchronously inside Allow, RecordSuccess and RecordFailure.
type CircuitBreaker struct {
	mu                  sync.Mutex
	state               State
	consecutiveFailures uint
	// lastTrialAt marks the start of the current trial window: the moment the
	// circuit opened, the last half-open trial was allowed, or a half-open trial failed.
	lastTrialAt time.Time
	cfg         Config
	logger      *slog.Logger
}

// New creates a CircuitBreaker with the given logger and configuration.
// A nil logger falls back to slog.Default; a zero FailureThreshold is treated as 1;
// a nil Clock falls back to time.Now; a non-positive ProbeInterval defaults to 1s
// (otherwise an open circuit would admit every call, silently disabling the breaker).
func New(logger *slog.Logger, cfg Config) *CircuitBreaker {
	if cfg.FailureThreshold == 0 {
		cfg.FailureThreshold = 1
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.ProbeInterval <= 0 {
		cfg.ProbeInterval = time.Second
	}
	return &CircuitBreaker{
		state:  StateClosed,
		cfg:    cfg,
		logger: logging.Child(logger, "circuit-breaker"),
	}
}

// Allow reports whether a call may proceed: true when the circuit is closed; when open
// (or half-open with no recorded result), true once per ProbeInterval window - i.e. it
// returns true and transitions to half-open if ProbeInterval has elapsed since opening
// or since the last trial, else false.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen, StateHalfOpen:
		now := cb.cfg.Clock()
		if now.Sub(cb.lastTrialAt) >= cb.cfg.ProbeInterval {
			cb.transitionTo(StateHalfOpen)
			cb.lastTrialAt = now
			return true
		}
		return false
	default:
		return false
	}
}

// RecordSuccess resets the consecutive failure count and closes the circuit from any state.
// Note that an in-flight call that began before the circuit opened can close it on
// completion without going through a half-open trial - this is intentional: a fresh
// success is at least as strong a health signal as a trial would provide.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures = 0
	cb.transitionTo(StateClosed)
}

// RecordFailure increments the consecutive failure count and opens the circuit once the
// failure threshold is reached. A failure recorded while half-open reopens the circuit
// and restarts the trial window from this moment.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures++

	switch cb.state {
	case StateClosed:
		if cb.consecutiveFailures >= cb.cfg.FailureThreshold {
			cb.transitionTo(StateOpen)
			cb.lastTrialAt = cb.cfg.Clock()
		}
	case StateHalfOpen:
		cb.transitionTo(StateOpen)
		cb.lastTrialAt = cb.cfg.Clock()
	case StateOpen:
		// Already open: keep the current trial window.
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// StartHealthProbe runs cfg.Probe every ProbeInterval while the circuit is open and
// closes the circuit when the probe succeeds. It is a no-op (returns immediately) when
// Probe is nil. It blocks until ctx is done - callers should run it in a goroutine.
func (cb *CircuitBreaker) StartHealthProbe(ctx context.Context) {
	if cb.cfg.Probe == nil {
		return
	}

	// ProbeInterval is guaranteed positive - New defaults it to 1s.
	ticker := time.NewTicker(cb.cfg.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if cb.State() != StateOpen {
				continue
			}
			if cb.cfg.Probe(ctx) {
				cb.logger.InfoContext(ctx, "health probe succeeded, closing circuit")
				cb.RecordSuccess()
			}
		}
	}
}

// transitionTo changes the state and logs the transition. Callers must hold cb.mu.
func (cb *CircuitBreaker) transitionTo(state State) {
	if cb.state == state {
		return
	}
	cb.logger.Info("circuit breaker state change",
		slog.String("from", cb.state.String()),
		slog.String("to", state.String()),
	)
	cb.state = state
}
