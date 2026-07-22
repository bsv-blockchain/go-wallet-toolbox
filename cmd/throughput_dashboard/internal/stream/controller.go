package stream

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/go-softwarelab/common/pkg/to"
	"golang.org/x/time/rate"
)

// ActionCreator is the wallet surface used by the stream.
type ActionCreator interface {
	CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error)
}

// Stats holds aggregate createAction outcomes.
type Stats struct {
	Attempted uint64 `json:"attempted"`
	Succeeded uint64 `json:"succeeded"`
	Failed    uint64 `json:"failed"`
	Iteration uint64 `json:"iteration"`
	Running   bool   `json:"running"`
	TPS       int    `json:"tps"`
	Workers   int    `json:"workers"`
	StartedAt string `json:"started_at,omitempty"`
}

// Options configure a stream run.
type Options struct {
	TPS        int
	Workers    int
	Originator string
}

// Hard caps for stream knobs. Workers sizes a channel buffer and a goroutine
// pool; unbounded user input would allow a denial-of-service allocation.
const (
	MaxTPS     = 100_000
	MaxWorkers = 512
)

// Controller is a start/stop controllable rate-limited createAction stream.
type Controller struct {
	wallet ActionCreator
	logger *slog.Logger

	mu         sync.Mutex
	running    bool
	cancel     context.CancelFunc
	done       chan struct{}
	tps        int
	workers    int
	originator string
	startedAt  time.Time

	attempted atomic.Uint64
	succeeded atomic.Uint64
	failed    atomic.Uint64
	iteration atomic.Uint64
}

// NewController creates a stream controller. Default options apply until Start overrides them.
func NewController(wallet ActionCreator, defaults Options, logger *slog.Logger) *Controller {
	if logger == nil {
		logger = slog.Default()
	}
	if defaults.TPS <= 0 {
		defaults.TPS = 10
	}
	if defaults.Workers <= 0 {
		defaults.Workers = 8
	}
	if defaults.Originator == "" {
		defaults.Originator = "throughput-dashboard.local"
	}
	return &Controller{
		wallet:     wallet,
		logger:     logger,
		tps:        defaults.TPS,
		workers:    defaults.Workers,
		originator: defaults.Originator,
	}
}

// Start begins the event stream. Returns an error if a stream is already running.
func (c *Controller) Start(parent context.Context, opts Options) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return fmt.Errorf("stream already running")
	}
	if opts.TPS > 0 {
		c.tps = opts.TPS
	}
	if opts.Workers > 0 {
		c.workers = opts.Workers
	}
	if opts.Originator != "" {
		c.originator = opts.Originator
	}
	if c.tps <= 0 || c.workers <= 0 {
		return fmt.Errorf("invalid stream options: tps=%d workers=%d", c.tps, c.workers)
	}
	if c.tps > MaxTPS {
		return fmt.Errorf("tps %d exceeds max %d", c.tps, MaxTPS)
	}
	if c.workers > MaxWorkers {
		return fmt.Errorf("workers %d exceeds max %d", c.workers, MaxWorkers)
	}

	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel
	c.done = make(chan struct{})
	c.running = true
	c.startedAt = time.Now().UTC()

	// Snapshot knobs for this run so concurrent Stats/Start readers cannot observe
	// mid-run mutation (Start rejects when running, but we still avoid racing reads).
	tps := c.tps
	workers := c.workers
	originator := c.originator
	done := c.done

	go c.run(ctx, done, tps, workers, originator)
	c.logger.Info("event stream started", "tps", tps, "workers", workers)
	return nil
}

// Stop requests stream shutdown and waits for workers to drain.
// Concurrent Stop calls are safe; a Stop that loses a race with a subsequent Start
// will not tear down the newer run.
func (c *Controller) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	cancel := c.cancel
	done := c.done
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}

	c.mu.Lock()
	// Only clear state if this Stop still owns the generation it cancelled.
	// run() may already have cleared running; a newer Start may have replaced done.
	if c.done == done {
		c.running = false
		c.cancel = nil
		c.done = nil
	}
	c.mu.Unlock()
	c.logger.Info("event stream stopped", "stats", c.Stats())
}

// Running reports whether the stream is active.
func (c *Controller) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Stats returns a snapshot of counters and run state.
func (c *Controller) Stats() Stats {
	c.mu.Lock()
	running := c.running
	tps := c.tps
	workers := c.workers
	started := c.startedAt
	c.mu.Unlock()

	s := Stats{
		Attempted: c.attempted.Load(),
		Succeeded: c.succeeded.Load(),
		Failed:    c.failed.Load(),
		Iteration: c.iteration.Load(),
		Running:   running,
		TPS:       tps,
		Workers:   workers,
	}
	if !started.IsZero() {
		s.StartedAt = started.Format(time.RFC3339Nano)
	}
	return s
}

// SnapshotAndDelta returns current stats and deltas since prev counters.
func (c *Controller) SnapshotAndDelta(prevAttempted, prevSucceeded, prevFailed uint64) (Stats, uint64, uint64, uint64) {
	s := c.Stats()
	return s, s.Attempted - prevAttempted, s.Succeeded - prevSucceeded, s.Failed - prevFailed
}

func (c *Controller) run(ctx context.Context, done chan struct{}, tps, workers int, originator string) {
	defer close(done)

	jobs := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				c.runOne(ctx, originator)
			}
		}()
	}

	limiter := rate.NewLimiter(rate.Limit(tps), 1)
	for {
		if err := limiter.Wait(ctx); err != nil {
			break
		}
		select {
		case <-ctx.Done():
			goto drain
		case jobs <- struct{}{}:
		}
	}
drain:
	close(jobs)
	wg.Wait()

	c.mu.Lock()
	// Mark stopped if this generation is still current. Stop() may also clear.
	if c.done == done {
		c.running = false
		c.cancel = nil
	}
	c.mu.Unlock()
}

func (c *Controller) runOne(ctx context.Context, originator string) {
	iter := c.iteration.Add(1)
	c.attempted.Add(1)

	locking, err := OpReturnLockingScriptForIteration(iter, time.Now().UTC())
	if err != nil {
		c.failed.Add(1)
		c.logger.Warn("opreturn build failed", "error", err, "iteration", iter)
		return
	}

	args := sdk.CreateActionArgs{
		Description: fmt.Sprintf("throughput dashboard stream #%d", iter),
		Outputs: []sdk.CreateActionOutput{
			{
				LockingScript:     locking,
				Satoshis:          0,
				OutputDescription: "sha256(iteration||timestamp)",
			},
		},
		Options: &sdk.CreateActionOptions{
			AcceptDelayedBroadcast: to.Ptr(true),
		},
	}

	if _, err := c.wallet.CreateAction(ctx, args, originator); err != nil {
		n := c.failed.Add(1)
		if n <= 5 || n%100 == 0 {
			c.logger.Warn("createAction failed", "error", err, "failed_count", n, "iteration", iter)
		}
		return
	}
	c.succeeded.Add(1)
}
