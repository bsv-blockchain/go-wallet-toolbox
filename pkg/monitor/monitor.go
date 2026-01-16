package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor/internal/tasks"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
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

	communicationChannels CommunicationChannels
}

type CommunicationChannels struct {
	OnTxBroadcasted chan<- defs.MonitorTaskResponse
	OnTxProven      chan<- defs.MonitorTaskResponse
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
func NewDaemonWithGORMLocker(ctx context.Context, logger *slog.Logger, storage MonitoredStorage, db *gorm.DB, opts ...CommunicationOption) (*Daemon, error) {
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

	options := defaultDaemonCommunicationOptions()
	for _, opt := range opts {
		opt(options)
	}

	return NewDaemon(logger.With(slog.String("worker", workerName)), storage, options, gocron.WithDistributedLocker(locker))
}

// NewDaemon creates a new Daemon instance with the provided logger and scheduler options.
// NOTE: To use a distributed scheduler, you need to provide a locker in the scheduler options or use NewDaemonWithGORMLocker.
func NewDaemon(logger *slog.Logger, storage MonitoredStorage, communicationOptions *DaemonCommunicationOptions, schedulerOptions ...gocron.SchedulerOption) (*Daemon, error) {
	scheduler, err := gocron.NewScheduler(schedulerOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	return &Daemon{
		scheduler:   scheduler,
		logger:      logging.Child(logger, "monitor"),
		activeTasks: make(map[defs.MonitorTask]*ActiveTask),
		storage:     storage,
		communicationChannels: CommunicationChannels{
			OnTxBroadcasted: communicationOptions.onTxBroadcasted,
			OnTxProven:      communicationOptions.onTxProven,
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
