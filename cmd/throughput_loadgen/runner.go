package main

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"golang.org/x/time/rate"
)

// Sample createAction failures: log the first N and every Nth thereafter.
// Keeps the high-TPS path light (no log on every failure).
const (
	createActionSampleFirst = 5
	createActionSampleEvery = 100
)

func shouldSampleCreateActionError(failedCount uint64) bool {
	return failedCount <= createActionSampleFirst || failedCount%createActionSampleEvery == 0
}

// ActionCreator is the wallet surface used by the load runner.
type ActionCreator interface {
	CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error)
}

// Stats holds aggregate createAction outcomes for a load run.
type Stats struct {
	Attempted uint64
	Succeeded uint64
	Failed    uint64
}

// loadCounters tracks in-flight loadgen outcomes.
type loadCounters struct {
	attempted atomic.Uint64
	succeeded atomic.Uint64
	failed    atomic.Uint64
}

func (c *loadCounters) snapshot() Stats {
	return Stats{
		Attempted: c.attempted.Load(),
		Succeeded: c.succeeded.Load(),
		Failed:    c.failed.Load(),
	}
}

// workerPool is the shared state for concurrent createAction workers.
type workerPool struct {
	ctx           context.Context
	cfg           Config
	wallet        ActionCreator
	lockingScript []byte
	jobs          <-chan struct{}
	counters      *loadCounters
}

// RunLoad issues rate-limited createAction calls until ctx is done or
// cfg.DurationSeconds elapses. Each action has a single OP_RETURN output
// with the provided locking script and Satoshis: 0.
//
// Duration expiry stops new work from being scheduled; in-flight actions still
// use the parent ctx so they can finish cleanly. Parent ctx cancellation
// (e.g. SIGINT) aborts both production and in-flight CreateAction calls.
func RunLoad(ctx context.Context, w ActionCreator, cfg Config, lockingScript []byte) Stats {
	produceCtx, stopProduce := produceContext(ctx, cfg.DurationSeconds)
	defer stopProduce()

	counters := &loadCounters{}
	jobs := make(chan struct{}, cfg.Workers)
	pool := workerPool{
		ctx:           ctx,
		cfg:           cfg,
		wallet:        w,
		lockingScript: lockingScript,
		jobs:          jobs,
		counters:      counters,
	}

	var wg sync.WaitGroup
	pool.start(&wg)

	logCancel, logDone := startProgressLogger(ctx, produceCtx, counters)
	produceJobs(produceCtx, rate.NewLimiter(rate.Limit(cfg.TPS), 1), jobs)
	close(jobs)
	wg.Wait()
	logCancel()
	<-logDone

	return counters.snapshot()
}

func produceContext(ctx context.Context, durationSeconds int) (context.Context, context.CancelFunc) {
	if durationSeconds <= 0 {
		// No duration limit: return parent ctx and a no-op cancel.
		return ctx, func() {
			// Unlimited run — nothing to cancel beyond the parent context.
		}
	}
	return context.WithTimeout(ctx, time.Duration(durationSeconds)*time.Second)
}

func (p *workerPool) start(wg *sync.WaitGroup) {
	for i := 0; i < p.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range p.jobs {
				p.runOne()
			}
		}()
	}
}

func (p *workerPool) runOne() {
	p.counters.attempted.Add(1)
	args := sdk.CreateActionArgs{
		Description: "throughput loadgen",
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     p.lockingScript,
				Satoshis:          0,
				OutputDescription: "throughput loadgen opreturn",
			},
		},
		Options: &sdk.CreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr(true),
		},
	}
	// Parent ctx so duration timeout only stops scheduling, not in-flight calls.
	if _, err := p.wallet.CreateAction(p.ctx, args, p.cfg.Originator); err != nil {
		n := p.counters.failed.Add(1)
		if shouldSampleCreateActionError(n) {
			slog.Warn("createAction failed", "error", err, "failed_count", n)
		}
		return
	}
	p.counters.succeeded.Add(1)
}

func startProgressLogger(
	ctx, produceCtx context.Context,
	counters *loadCounters,
) (context.CancelFunc, <-chan struct{}) {
	logCtx, logCancel := context.WithCancel(ctx)
	logDone := make(chan struct{})
	go func() {
		defer close(logDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var prevAttempted uint64
		for {
			select {
			case <-logCtx.Done():
				return
			case <-produceCtx.Done():
				return
			case <-ticker.C:
				cur := counters.attempted.Load()
				slog.Info("loadgen progress",
					"attempted", cur,
					"succeeded", counters.succeeded.Load(),
					"failed", counters.failed.Load(),
					"rate_per_s", cur-prevAttempted,
				)
				prevAttempted = cur
			}
		}
	}()
	return logCancel, logDone
}

func produceJobs(produceCtx context.Context, limiter *rate.Limiter, jobs chan<- struct{}) {
	for {
		if err := limiter.Wait(produceCtx); err != nil {
			return
		}
		select {
		case <-produceCtx.Done():
			return
		case jobs <- struct{}{}:
		}
	}
}
