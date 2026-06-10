package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arc"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/arcade"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/bitails"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/circuitbreaker"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/internal/whatsonchain"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// defaultMaxBackpressureWait bounds how long the router honors an Arcade
// Retry-After hint before retrying the primary broadcast.
const defaultMaxBackpressureWait = 30 * time.Second

// broadcastTarget is a single named broadcaster the router can use.
// post receives both the EF hex and the raw tx bytes so that targets with
// different broadcast inputs (PostEF vs PostTX) fit the same shape.
// post MUST return a non-nil result when err is nil.
type broadcastTarget struct {
	name string
	post func(ctx context.Context, efHex string, rawTx []byte, txID string) (*wdk.PostedTxID, error)
}

// broadcastRouter routes each broadcast to the primary (Arcade) and fails over
// to the ordered failover targets only when the primary transport fails or its
// circuit breaker is open. A tx-level rejection (err == nil with an error in
// the result) is a final answer and never triggers failover.
type broadcastRouter struct {
	logger    *slog.Logger
	primary   broadcastTarget
	breaker   *circuitbreaker.CircuitBreaker
	failovers []broadcastTarget
	// maxBackpressureWait bounds honored Retry-After delays (default 30s).
	maxBackpressureWait time.Duration
	// sleep is the context-aware wait used for backpressure; injectable for tests.
	sleep func(ctx context.Context, d time.Duration)
}

// newBroadcastRouter assembles the router with Arcade as the primary and the
// fixed failover order TAAL ARC -> GorillaPool ARC -> WhatsOnChain -> Bitails,
// skipping services that are not configured (nil).
func newBroadcastRouter(
	logger *slog.Logger,
	breaker *circuitbreaker.CircuitBreaker,
	arcadeService *arcade.Service,
	arcService *arc.Service,
	arcGorillaPoolService *arc.Service,
	wocService *whatsonchain.WhatsOnChain,
	bitailsService *bitails.Bitails,
) *broadcastRouter {
	router := &broadcastRouter{
		logger:  logging.Child(logger, "broadcast-router"),
		breaker: breaker,
		primary: broadcastTarget{
			name: defs.ArcadeServiceName,
			post: func(ctx context.Context, efHex string, _ []byte, txID string) (*wdk.PostedTxID, error) {
				return arcadeService.PostEF(ctx, efHex, txID)
			},
		},
		maxBackpressureWait: defaultMaxBackpressureWait,
		sleep:               sleepWithContext,
	}

	if arcService != nil {
		router.failovers = append(router.failovers, arcFailoverTarget(arcService))
	}
	if arcGorillaPoolService != nil {
		router.failovers = append(router.failovers, arcFailoverTarget(arcGorillaPoolService))
	}
	if wocService != nil {
		router.failovers = append(router.failovers, broadcastTarget{
			name: whatsonchain.ServiceName,
			post: func(ctx context.Context, _ string, rawTx []byte, _ string) (*wdk.PostedTxID, error) {
				return resultAsTransportOutcome(wocService.PostTX(ctx, rawTx))
			},
		})
	}
	if bitailsService != nil {
		router.failovers = append(router.failovers, broadcastTarget{
			name: bitails.ServiceName,
			post: func(ctx context.Context, _ string, rawTx []byte, _ string) (*wdk.PostedTxID, error) {
				return resultAsTransportOutcome(bitailsService.PostTX(ctx, rawTx))
			},
		})
	}

	return router
}

// broadcast routes one tx. It returns results for every service actually
// attempted (happy path: exactly one - Arcade). It never returns an empty
// slice: when nothing can be attempted (circuit open, no failovers configured)
// a final error result for the primary is returned instead.
func (r *broadcastRouter) broadcast(ctx context.Context, efHex string, rawTx []byte, txID string) []*wdk.PostFromBEEFServiceResult {
	var results []*wdk.PostFromBEEFServiceResult

	if r.breaker.Allow() {
		posted, err := r.primary.post(ctx, efHex, rawTx, txID)

		var backpressure *arcade.BackpressureError
		if errors.As(err, &backpressure) {
			wait := min(backpressure.RetryAfter, r.maxBackpressureWait)
			r.logger.InfoContext(ctx, "primary broadcaster applied backpressure, retrying once",
				slog.String("txID", txID),
				slog.Duration("wait", wait),
			)
			r.sleep(ctx, wait)
			if ctxErr := ctx.Err(); ctxErr != nil {
				// the caller went away while the router honored backpressure: report it
				// without retrying and without poisoning the circuit breaker (Arcade
				// itself may be perfectly healthy).
				return append(results, &wdk.PostFromBEEFServiceResult{
					Name:  r.primary.name,
					Error: fmt.Errorf("broadcast canceled while honoring backpressure: %w", ctxErr),
				})
			}
			posted, err = r.primary.post(ctx, efHex, rawTx, txID)
		}

		if err == nil {
			r.breaker.RecordSuccess()
			return []*wdk.PostFromBEEFServiceResult{targetResult(r.primary.name, posted)}
		}

		results = append(results, &wdk.PostFromBEEFServiceResult{Name: r.primary.name, Error: err})

		if isContextError(err) {
			// the caller went away mid-broadcast: not a service failure, so it must
			// not count against the circuit breaker. With a dead context the
			// failovers cannot succeed either, so stop here.
			r.logger.InfoContext(ctx, "primary broadcast canceled by the caller",
				slog.String("txID", txID),
				slog.String("service", r.primary.name),
				slog.String("error", err.Error()),
			)
			if ctx.Err() != nil {
				return results
			}
		} else {
			// transport failure (or repeated backpressure): count it and fail over
			r.breaker.RecordFailure()
			r.logger.WarnContext(ctx, "primary broadcaster failed, failing over",
				slog.String("txID", txID),
				slog.String("service", r.primary.name),
				slog.String("error", err.Error()),
			)
		}
	} else {
		r.logger.WarnContext(ctx, "primary broadcaster circuit is open, failing over",
			slog.String("txID", txID),
			slog.String("service", r.primary.name),
		)
	}

	for _, target := range r.failovers {
		posted, err := target.post(ctx, efHex, rawTx, txID)
		if err == nil {
			results = append(results, targetResult(target.name, posted))
			return results
		}
		r.logger.WarnContext(ctx, "failover broadcaster failed",
			slog.String("txID", txID),
			slog.String("service", target.name),
			slog.String("error", err.Error()),
		)
		results = append(results, &wdk.PostFromBEEFServiceResult{Name: target.name, Error: err})
	}

	if len(results) == 0 {
		// circuit open and no failover targets configured: never return zero results,
		// the storage layer needs an error result to keep the tx retryable.
		results = append(results, &wdk.PostFromBEEFServiceResult{
			Name:  r.primary.name,
			Error: errors.New("broadcast skipped: circuit open and no failover targets configured"),
		})
	}

	return results
}

// isContextError reports whether err was caused by context cancellation or an
// exceeded deadline - i.e. the caller went away, which says nothing about the
// health of the broadcaster.
func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// arcFailoverTarget adapts an ARC service for the failover chain, routing its
// result through resultAsTransportOutcome.
func arcFailoverTarget(svc *arc.Service) broadcastTarget {
	return broadcastTarget{
		name: svc.Name(),
		post: func(ctx context.Context, efHex string, _ []byte, txID string) (*wdk.PostedTxID, error) {
			return resultAsTransportOutcome(svc.PostEF(ctx, efHex, txID))
		},
	}
}

// resultAsTransportOutcome converts a result-level error into a Go error so
// that the failover chain keeps going. arc.PostEF, whatsonchain.PostTX and
// bitails.PostTX never return a Go error - they fold every failure (including
// transport errors like "service is unreachable") into the result's Error
// field, which would otherwise wrongly "win" the chain (err == nil stops it).
//
// A DoubleSpend verdict is the one exception: it is a final answer about the
// tx itself, not a service failure. Hint verdicts too (e.g. WhatsOnChain's
// missing-inputs answer sets DoubleSpend with no Error) intentionally stop
// the chain here - they are fed into the confirmDoubleSpends verification
// downstream, and broadcasting a possibly-conflicting tx to even more
// services would be exactly the spraying this design forbids.
func resultAsTransportOutcome(posted *wdk.PostedTxID, err error) (*wdk.PostedTxID, error) {
	if err != nil {
		return nil, err
	}
	if posted != nil && posted.Error != nil && !posted.DoubleSpend {
		return nil, posted.Error
	}
	return posted, nil
}

// targetResult wraps a single posted-tx result into a named service result.
// A nil result with no error breaks the broadcastTarget contract; it is folded
// into an error result instead of panicking.
func targetResult(name string, posted *wdk.PostedTxID) *wdk.PostFromBEEFServiceResult {
	if posted == nil {
		return &wdk.PostFromBEEFServiceResult{Name: name, Error: errors.New("service returned no result")}
	}
	return &wdk.PostFromBEEFServiceResult{
		Name:             name,
		PostedBEEFResult: &wdk.PostedBEEF{TxIDResults: []wdk.PostedTxID{*posted}},
	}
}

// sleepWithContext waits for d or until ctx is done, whichever comes first.
func sleepWithContext(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
