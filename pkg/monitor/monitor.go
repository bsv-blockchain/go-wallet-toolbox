package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	gormlock "github.com/go-co-op/gocron-gorm-lock/v2"
	"github.com/go-co-op/gocron/v2"
	"gorm.io/gorm"
)

const safetyMargin = 0.95 // Safety margin to ensure tasks complete before the next scheduled run

// Daemon is responsible for scheduling and running monitoring tasks at specified intervals.
// It uses a distributed scheduler to ensure tasks are run reliably across multiple instances.
type Daemon struct {
	scheduler   gocron.Scheduler
	logger      *slog.Logger
	activeTasks map[defs.MonitorTask]*ActiveTask

	storage MonitoredStorage

	started   bool
	startLock sync.Mutex

	eventChannels EventChannels
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
func NewDaemonWithGORMLocker(ctx context.Context, logger *slog.Logger, storage MonitoredStorage, db *gorm.DB, opts ...DaemonEventOption) (*Daemon, error) {
	err := db.WithContext(ctx).AutoMigrate(gormlock.CronJobLock{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate cronjob table: %w", err)
	}

	workerName, err := randomizer.New().Base64(12)
	if err != nil {
		return nil, fmt.Errorf("failed to generate worker name: %w", err)
	}
	locker, err := gormlock.NewGormLocker(db, workerName, gormlock.WithDefaultJobIdentifier(time.Millisecond))
	if err != nil {
		return nil, fmt.Errorf("failed to create gorm locker: %w", err)
	}

	options := defaultDaemonEventOptions()
	for _, opt := range opts {
		opt(options)
	}

	return NewDaemon(logger.With(slog.String("worker", workerName)), storage, options, gocron.WithDistributedLocker(locker))
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
	}, nil
}

// Start initializes and begins running the configured monitor tasks according to their schedules.
func (d *Daemon) Start(tasksToStart map[defs.MonitorTask]defs.TaskConfig) error {
	d.startLock.Lock()
	defer d.startLock.Unlock()

	if d.started {
		d.logger.Warn("Daemon is already started. Skipping.")
		return nil
	}

	factories := d.allTasksFactories()
	for taskName, taskConfig := range tasksToStart {
		taskFactory, ok := factories[taskName]
		if !ok {
			d.logger.Warn("Task does not exist. Skipping.", slog.Any("task", taskName))
			continue
		}

		if err := d.initializeTask(taskFactory(), taskName, taskConfig); err != nil {
			return err
		}
	}

	if d.eventChannels.OnReorg != nil {
		go d.handleReorgEvents()
	}

	if d.eventChannels.OnTip != nil {
		go d.handleNewTipEvents(context.Background())
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
		d.logger.Warn("Daemon is not started. Skipping.")
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
		d.logger.Warn("Daemon is not started. Skipping.")
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

func (d *Daemon) initializeTask(taskInstance tasks.TaskInterface, taskName defs.MonitorTask, taskConfig defs.TaskConfig) error {
	task := &ActiveTask{
		Instance: taskInstance,
		TaskName: taskName,
		// NOTE: Cronjob (gocron.Job) is not set here, as it will be set when the job is created.
	}

	opts := []gocron.JobOption{
		gocron.WithName(fmt.Sprintf("monitor_%s", taskName)),
	}

	if taskConfig.StartImmediately {
		opts = append(opts, gocron.WithStartAt(gocron.WithStartImmediately()))
	}

	interval := taskConfig.Interval()

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

	d.logger.Info("Starting a task", "task", taskName, "interval", interval, "start_immediately", taskConfig.StartImmediately)
	return nil
}

func (d *Daemon) singleTaskRunner(activeTask *ActiveTask) func(ctx context.Context) {
	return func(ctx context.Context) {
		var err error
		ctx, span := tracing.StartTracing(ctx, fmt.Sprintf("Task-%s", activeTask.TaskName))
		defer func() {
			tracing.EndTracing(span, err)
		}()

		d.logger.Info("Run task", slog.Any("task", activeTask.TaskName))
		defer func() {
			if err != nil {
				d.logger.Error("Task failed", slog.Any("task", activeTask.TaskName), slog.Any("error", err))
				return
			}
			if activeTask.Cronjob == nil {
				return
			}
			nextRun, _ := activeTask.Cronjob.NextRun()
			d.logger.Info("Finish task", slog.Any("task", activeTask.TaskName), slog.Any("next_run", nextRun))
		}()

		nextRun, err := activeTask.Cronjob.NextRun()
		if err != nil {
			d.logger.Error("Failed to get next run for task", slog.Any("task", activeTask.TaskName), slog.Any("error", err))
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

func (d *Daemon) handleReorgEvents() {
	d.logger.Info("Starting reorg event handler")

	for event := range d.eventChannels.OnReorg {
		d.logger.Info("Received reorg event",
			"depth", event.Depth,
			"orphaned_count", len(event.OrphanedHashes),
		)

		orphanedHashes := make([]string, len(event.OrphanedHashes))
		for i, hash := range event.OrphanedHashes {
			orphanedHashes[i] = hash.String()
		}

		if err := d.storage.HandleReorg(context.Background(), orphanedHashes); err != nil {
			d.logger.Error("Failed to handle reorg", "error", err)
		}
	}

	d.logger.Info("reorg event handler stopped")
}

func (d *Daemon) handleNewTipEvents(ctx context.Context) {
	d.logger.Info("Starting new tip event handler")

	for header := range d.eventChannels.OnTip {
		d.logger.Info("New tip received and processing",
			"height", header.Height,
			"hash", header.Hash.String(),
		)

		go func(h *chaintracks.BlockHeader) {
			results, err := d.storage.ProcessNewTip(ctx, header.Height, header.Hash.String())
			if err != nil {
				d.logger.Error("ProcessNewTip failed", "error", err)
				return
			}

			d.sendProvenEvents(ctx, results)
		}(header)
	}
}

func (d *Daemon) sendProvenEvents(ctx context.Context, results []wdk.TxSynchronizedStatus) {
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
		}

		select {
		case d.eventChannels.OnTxProven <- msg:
		case <-ctx.Done():
			return
		default:
			d.logger.Warn("OnTxProven channel in monitor is full, dropping event")
		}
	}
}
