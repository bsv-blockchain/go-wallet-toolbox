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

	var attempted, succeeded, failed atomic.Uint64
	jobs := make(chan struct{}, cfg.Workers)

	var wg sync.WaitGroup
	startWorkers(ctx, &wg, cfg, w, lockingScript, jobs, &attempted, &succeeded, &failed)

	logCancel, logDone := startProgressLogger(ctx, produceCtx, &attempted, &succeeded, &failed)
	produceJobs(produceCtx, rate.NewLimiter(rate.Limit(cfg.TPS), 1), jobs)
	close(jobs)
	wg.Wait()
	logCancel()
	<-logDone

	return Stats{
		Attempted: attempted.Load(),
		Succeeded: succeeded.Load(),
		Failed:    failed.Load(),
	}
}

func produceContext(ctx context.Context, durationSeconds int) (context.Context, context.CancelFunc) {
	if durationSeconds <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, time.Duration(durationSeconds)*time.Second)
}

func startWorkers(
	ctx context.Context,
	wg *sync.WaitGroup,
	cfg Config,
	w ActionCreator,
	lockingScript []byte,
	jobs <-chan struct{},
	attempted, succeeded, failed *atomic.Uint64,
) {
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				runOneCreateAction(ctx, w, cfg.Originator, lockingScript, attempted, succeeded, failed)
			}
		}()
	}
}

func runOneCreateAction(
	ctx context.Context,
	w ActionCreator,
	originator string,
	lockingScript []byte,
	attempted, succeeded, failed *atomic.Uint64,
) {
	attempted.Add(1)
	args := sdk.CreateActionArgs{
		Description: "throughput loadgen",
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     lockingScript,
				Satoshis:          0,
				OutputDescription: "throughput loadgen opreturn",
			},
		},
		Options: &sdk.CreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr(true),
		},
	}
	// Parent ctx so duration timeout only stops scheduling, not in-flight calls.
	if _, err := w.CreateAction(ctx, args, originator); err != nil {
		n := failed.Add(1)
		if shouldSampleCreateActionError(n) {
			slog.Warn("createAction failed", "error", err, "failed_count", n)
		}
		return
	}
	succeeded.Add(1)
}

func startProgressLogger(
	ctx, produceCtx context.Context,
	attempted, succeeded, failed *atomic.Uint64,
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
				cur := attempted.Load()
				slog.Info("loadgen progress",
					"attempted", cur,
					"succeeded", succeeded.Load(),
					"failed", failed.Load(),
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
