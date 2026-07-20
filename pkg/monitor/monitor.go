package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/go-co-op/gocron/v2"
	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const safetyMargin = 0.95 // Safety margin to ensure tasks complete before the next scheduled run

// Daemon is responsible for scheduling and running monitoring tasks at specified intervals.
// It uses a distributed scheduler to ensure tasks are run reliably across multiple instances.
type Daemon struct {
	scheduler   gocron.Scheduler
	logger      *slog.Logger
	activeTasks map[defs.MonitorTask]*ActiveTask

	storage MonitoredStorage

	// leaseLocker is set only when the daemon was built via
	// NewDaemonWithGORMLocker; it is nil for a plain NewDaemon. When set,
	// Start wires a per-job lease TTL derived from each task's interval.
	leaseLocker *LeaseLocker

	started   bool
	startLock sync.Mutex

	eventChannels          EventChannels
	broadcastEventStreamer BroadcastEventStreamer
}

// EventChannels holds channels for bidirectional communication with the monitor.
// Outbound channels (chan<-) are used by monitor to send notifications back to other components.
// Inbound channels (<-chan) are used by monitor to receive external events.
type EventChannels struct {
	// Outbound channels:
	OnTxBroadcasted chan<- wdk.CurrentTxStatus
	OnTxProven      chan<- wdk.CurrentTxStatus

	// Inbound channels:
	OnReorg <-chan *chaintracks.ReorgEvent
	OnTip   <-chan *chaintracks.BlockHeader
}

// ActiveTask represents a scheduled monitoring task with its instance and associated scheduler job.
// It holds the task logic and the job entry created in the distributed scheduler for management purposes.
type ActiveTask struct {
	Instance tasks.TaskInterface
	Cronjob  gocron.Job
	TaskName defs.MonitorTask
}

// NewDaemonWithGORMLocker creates a new Daemon instance with a GORM-based distributed lock.
// This ensures that scheduled tasks run on only one instance when multiple application instances are deployed.
//
// The lock is a lease per job (see LeaseLocker): every instance contends on the
// same stable per-job key, so exactly one instance runs a job at a time and a
// crashed owner is reclaimed once its lease expires. Per-job lease TTLs are
// wired from task intervals in Start.
func NewDaemonWithGORMLocker(ctx context.Context, logger *slog.Logger, storage MonitoredStorage, db *gorm.DB, opts ...DaemonEventOption) (*Daemon, error) {
	workerName, err := randomizer.New().Base64(12)
	if err != nil {
		return nil, fmt.Errorf("failed to generate worker name: %w", err)
	}

	workerLogger := logger.With(slog.String("worker", workerName))

	locker, err := NewLeaseLocker(db.WithContext(ctx), workerName, workerLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create lease locker: %w", err)
	}

	options := defaultDaemonEventOptions()
	for _, opt := range opts {
		opt(options)
	}

	daemon, err := NewDaemon(workerLogger, storage, options, gocron.WithDistributedLocker(locker))
	if err != nil {
		return nil, err
	}
	daemon.leaseLocker = locker

	return daemon, nil
}

// NewDaemon creates a new Daemon instance with the provided logger and scheduler options.
// NOTE: To use a distributed scheduler, you need to provide a locker in the scheduler options or use NewDaemonWithGORMLocker.
func NewDaemon(logger *slog.Logger, storage MonitoredStorage, eventOptions *DaemonEventOptions, schedulerOptions ...gocron.SchedulerOption) (*Daemon, error) {
	scheduler, err := gocron.NewScheduler(schedulerOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	return &Daemon{
		scheduler:   scheduler,
		logger:      logging.Child(logger, "monitor"),
		activeTasks: make(map[defs.MonitorTask]*ActiveTask),
		storage:     storage,
		eventChannels: EventChannels{
			OnTxBroadcasted: eventOptions.onTxBroadcasted,
			OnTxProven:      eventOptions.onTxProven,
			OnReorg:         eventOptions.onReorg,
			OnTip:           eventOptions.onTip,
		},
		broadcastEventStreamer: eventOptions.broadcastEventStreamer,
	}, nil
}

// Start initializes and begins running the configured monitor tasks according to their schedules.
func (d *Daemon) Start(ctx context.Context, tasksToStart map[defs.MonitorTask]defs.TaskConfig) error {
	d.startLock.Lock()
	defer d.startLock.Unlock()

	if d.started {
		d.logger.WarnContext(ctx, "Daemon is already started. Skipping.")
		return nil
	}

	factories := d.allTasksFactories()
	for taskName, taskConfig := range tasksToStart {
		taskFactory, ok := factories[taskName]
		if !ok {
			d.logger.WarnContext(ctx, "Task does not exist. Skipping.", slog.Any("task", taskName))
			continue
		}

		if err := d.initializeTask(taskFactory(), taskName, taskConfig); err != nil {
			return err
		}
	}

	if d.eventChannels.OnReorg != nil {
		go d.handleReorgEvents(ctx)
	}

	if d.eventChannels.OnTip != nil {
		go d.handleNewTipEvents(ctx)
	}

	if d.broadcastEventStreamer != nil {
		go d.handleBroadcastEvents(ctx, d.broadcastEventStreamer)
	}

	d.scheduler.Start()
	d.started = true
	return nil
}

// Pause stops all scheduled jobs if the daemon is currently running.
// If the daemon is not started, it logs a warning and does nothing.
// Returns an error if stopping the jobs fails.
func (d *Daemon) Pause() error {
	d.startLock.Lock()
	defer d.startLock.Unlock()

	if !d.started {
		d.logger.WarnContext(context.Background(), "Daemon is not started. Skipping.")
		return nil
	}

	err := d.scheduler.StopJobs()
	if err != nil {
		return fmt.Errorf("failed to stop jobs: %w", err)
	}
	return nil
}

// Stop shuts down the daemon, releasing all resources and clearing scheduled jobs.
// If the daemon is not running, logs a warning and returns nil.
// The Daemon cannot be restarted after stopping.
func (d *Daemon) Stop() error {
	d.startLock.Lock()
	defer d.startLock.Unlock()

	if !d.started {
		d.logger.WarnContext(context.Background(), "Daemon is not started. Skipping.")
		return nil
	}

	err := d.scheduler.Shutdown()
	if err != nil {
		return fmt.Errorf("failed to clear jobs: %w", err)
	}
	return nil
}

// Get retrieves the active monitoring task associated with the given name.
// Returns the ActiveTask pointer and true if found, otherwise nil and false.
func (d *Daemon) Get(name defs.MonitorTask) (*ActiveTask, bool) {
	task, ok := d.activeTasks[name]
	return task, ok
}

// monitorJobName is the gocron job name for a task. gocron passes this exact
// string to the distributed locker as the lock key, so lease TTLs must be
// registered under the same name (see the SetLeaseTTL call in initializeTask).
func monitorJobName(taskName defs.MonitorTask) string {
	return fmt.Sprintf("monitor_%s", taskName)
}

func (d *Daemon) initializeTask(taskInstance tasks.TaskInterface, taskName defs.MonitorTask, taskConfig defs.TaskConfig) error {
	task := &ActiveTask{
		Instance: taskInstance,
		TaskName: taskName,
		// NOTE: Cronjob (gocron.Job) is not set here, as it will be set when the job is created.
	}

	jobName := monitorJobName(taskName)

	opts := []gocron.JobOption{
		gocron.WithName(jobName),
		// gocron runs each tick in a fresh goroutine and does NOT prevent a
		// single process from overlapping runs of the same job. Without this,
		// a run overrunning its interval would re-acquire its own lease
		// (owner=me) and overlap itself; when the first run Unlocks
		// (lease_until=now) another pod could claim mid-flight — a narrow
		// break of exactly-once. Singleton mode makes the same-process
		// non-overlap premise the lease relies on actually true; overlapping
		// ticks are rescheduled rather than run concurrently.
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	}

	if taskConfig.StartImmediately {
		opts = append(opts, gocron.WithStartAt(gocron.WithStartImmediately()))
	}

	interval := taskConfig.Interval()

	// Wire the lease TTL for this job before it can run. The lock key gocron
	// uses is jobName, so the TTL must be registered under jobName. A TTL of
	// max(2*interval, 5m) tolerates a run overrunning its slot and gives a
	// crashed owner enough slack that a healthy peer does not steal a job that
	// is merely slow, while still reclaiming within a bounded time.
	if d.leaseLocker != nil {
		ttl := 2 * interval
		if ttl < 5*time.Minute {
			ttl = 5 * time.Minute
		}
		d.leaseLocker.SetLeaseTTL(jobName, ttl)
	}

	job, err := d.scheduler.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(d.singleTaskRunner(task)),
		opts...,
	)
	if err != nil {
		return fmt.Errorf("failed to create job %s: %w", taskName, err)
	}

	task.Cronjob = job
	d.activeTasks[taskName] = task

	d.logger.InfoContext(context.Background(), "Starting a task", "task", taskName, "interval", interval, "start_immediately", taskConfig.StartImmediately)
	return nil
}

func (d *Daemon) singleTaskRunner(activeTask *ActiveTask) func(ctx context.Context) {
	return func(ctx context.Context) {
		var err error
		ctx, span := tracing.StartTracing(ctx, fmt.Sprintf("Task-%s", activeTask.TaskName))
		defer func() {
			tracing.EndTracing(span, err)
		}()

		d.logger.InfoContext(ctx, "Run task", slog.Any("task", activeTask.TaskName))
		defer func() {
			if err != nil {
				d.logger.ErrorContext(ctx, "Task failed", slog.Any("task", activeTask.TaskName), slog.Any("error", err))
				return
			}
			if activeTask.Cronjob == nil {
				return
			}
			nextRun, _ := activeTask.Cronjob.NextRun()
			d.logger.InfoContext(ctx, "Finish task", slog.Any("task", activeTask.TaskName), slog.Any("next_run", nextRun))
		}()

		nextRun, err := activeTask.Cronjob.NextRun()
		if err != nil {
			d.logger.ErrorContext(ctx, "Failed to get next run for task", slog.Any("task", activeTask.TaskName), slog.Any("error", err))
			return
		}

		ctx, cancel := d.contextWithTimeout(ctx, nextRun)
		defer cancel()

		err = activeTask.Instance.Run(ctx)
	}
}

func (d *Daemon) contextWithTimeout(ctx context.Context, nextRun time.Time) (context.Context, context.CancelFunc) {
	if nextRun.IsZero() {
		return ctx, func() {}
	}

	now := time.Now()
	untilNext := nextRun.Sub(now)

	if untilNext <= 0 {
		return ctx, func() {}
	}

	timeout := time.Duration(float64(untilNext) * safetyMargin)
	return context.WithTimeout(ctx, timeout)
}

func (d *Daemon) handleReorgEvents(ctx context.Context) {
	d.logger.InfoContext(ctx, "Starting reorg event handler")

	for event := range d.eventChannels.OnReorg {
		d.logger.InfoContext(
			ctx, "Received reorg event",
			"depth", event.Depth,
			"orphaned_count", len(event.OrphanedHashes),
		)

		orphanedHashes := make([]string, len(event.OrphanedHashes))
		for i, hash := range event.OrphanedHashes {
			orphanedHashes[i] = hash.String()
		}

		if err := d.storage.HandleReorg(ctx, orphanedHashes); err != nil {
			d.logger.ErrorContext(ctx, "Failed to handle reorg", "error", err)
		}
	}

	d.logger.InfoContext(ctx, "reorg event handler stopped")
}

func (d *Daemon) handleNewTipEvents(ctx context.Context) {
	d.logger.InfoContext(ctx, "Starting new tip event handler")

	for header := range d.eventChannels.OnTip {
		d.logger.InfoContext(
			ctx, "New tip received and processing",
			"height", header.Height,
			"hash", header.Hash.String(),
		)

		go func(h *chaintracks.BlockHeader) {
			results, err := d.storage.ProcessNewTip(ctx, h.Height, h.Hash.String())
			if err != nil {
				d.logger.ErrorContext(ctx, "ProcessNewTip failed", "error", err)
				return
			}

			d.sendProvenEvents(results)
		}(header)
	}
}

func (d *Daemon) sendProvenEvents(results []wdk.TxSynchronizedStatus) {
	if d.eventChannels.OnTxProven == nil {
		return
	}

	for _, res := range results {
		msg := wdk.CurrentTxStatus{
			TxID:        res.TxID,
			Status:      res.Status.ToStandardizedStatus(),
			MerklePath:  res.MerklePath,
			MerkleRoot:  res.MerkleRoot,
			BlockHash:   res.BlockHash,
			BlockHeight: res.BlockHeight,
			Reference:   res.Reference,
			Labels:      res.Labels,
		}

		select {
		case d.eventChannels.OnTxProven <- msg:
		default:
			d.logger.WarnContext(context.Background(), "OnTxProven channel in monitor is full, dropping event")
		}
	}
}
