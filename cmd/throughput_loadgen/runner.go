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
	produceCtx := ctx
	if cfg.DurationSeconds > 0 {
		var cancel context.CancelFunc
		produceCtx, cancel = context.WithTimeout(ctx, time.Duration(cfg.DurationSeconds)*time.Second)
		defer cancel()
	}

	var attempted, succeeded, failed atomic.Uint64

	limiter := rate.NewLimiter(rate.Limit(cfg.TPS), 1)
	jobs := make(chan struct{}, cfg.Workers)

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
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
				// Use parent ctx so duration timeout only stops scheduling.
				_, err := w.CreateAction(ctx, args, cfg.Originator)
				if err != nil {
					failed.Add(1)
					continue
				}
				succeeded.Add(1)
			}
		}()
	}

	logCtx, logCancel := context.WithCancel(context.Background())
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

produce:
	for {
		if err := limiter.Wait(produceCtx); err != nil {
			break
		}
		select {
		case <-produceCtx.Done():
			break produce
		case jobs <- struct{}{}:
		}
	}
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
