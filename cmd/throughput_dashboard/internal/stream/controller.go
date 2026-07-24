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

// WorkersForTPS chooses a worker pool that can sustain tps if each createAction
// takes on the order of one second end-to-end (storage RPC + wallet). The
// producer rate-limits at tps; workers are the concurrent in-flight budget.
// When workers < tps and latency is ~1s, the jobs channel backs up and achieved
// TPS falls. Result is clamped to [1, MaxWorkers].
func WorkersForTPS(tps int) int {
	if tps <= 0 {
		return 1
	}
	if tps > MaxWorkers {
		return MaxWorkers
	}
	return tps
}

// Controller is a start/stop controllable rate-limited createAction stream.
type Controller struct {
	wallet ActionCreator
	logger *slog.Logger

	mu         sync.Mutex
	running    bool
	stopProd   context.CancelFunc // stops new job production only
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
// When defaults.Workers is <= 0, workers are derived from TPS via WorkersForTPS.
func NewController(wallet ActionCreator, defaults Options, logger *slog.Logger) *Controller {
	if logger == nil {
		logger = slog.Default()
	}
	if defaults.TPS <= 0 {
		defaults.TPS = 10
	}
	if defaults.Workers <= 0 {
		defaults.Workers = WorkersForTPS(defaults.TPS)
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
//
// When opts.Workers is <= 0, the worker pool size is derived from the effective
// TPS (WorkersForTPS). Explicit Workers > 0 still override (API/tests).
//
// parent, when canceled, stops scheduling new events (same as Stop) but does not
// abort createActions already in flight. Stop always waits for those to finish.
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
	} else {
		c.workers = WorkersForTPS(c.tps)
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
	if parent == nil {
		parent = context.Background()
	}

	// prodCtx is canceled by Stop() or when parent ends — only the job producer
	// uses it. workCtx is never canceled so in-flight CreateAction calls finish.
	prodCtx, stopProd := context.WithCancel(parent)
	workCtx := context.WithoutCancel(parent)
	c.stopProd = stopProd
	c.done = make(chan struct{})
	c.running = true
	c.startedAt = time.Now().UTC()

	// Snapshot knobs for this run so concurrent Stats/Start readers cannot observe
	// mid-run mutation (Start rejects when running, but we still avoid racing reads).
	tps := c.tps
	workers := c.workers
	originator := c.originator
	done := c.done

	go c.run(prodCtx, workCtx, done, tps, workers, originator)
	c.logger.Info("event stream started", "tps", tps, "workers", workers)
	return nil
}

// Stop stops scheduling new createAction events and waits for in-flight ones to finish.
// It does not cancel wallet RPCs that have already started.
// Concurrent Stop calls are safe; a Stop that loses a race with a subsequent Start
// will not tear down the newer run.
func (c *Controller) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	stopProd := c.stopProd
	done := c.done
	c.mu.Unlock()

	if stopProd != nil {
		stopProd()
	}
	if done != nil {
		<-done
	}

	c.mu.Lock()
	// Only clear state if this Stop still owns the generation it stopped.
	// run() may already have cleared running; a newer Start may have replaced done.
	if c.done == done {
		c.running = false
		c.stopProd = nil
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

func (c *Controller) run(prodCtx, workCtx context.Context, done chan struct{}, tps, workers int, originator string) {
	defer close(done)

	jobs := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				// workCtx is never canceled by Stop — only production is halted.
				c.runOne(workCtx, originator)
			}
		}()
	}

	limiter := rate.NewLimiter(rate.Limit(tps), 1)
	for {
		if err := limiter.Wait(prodCtx); err != nil {
			break
		}
		select {
		case <-prodCtx.Done():
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
		c.stopProd = nil
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
