package storage

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/tasks"
	gormlock "github.com/go-co-op/gocron-gorm-lock/v2"
	"github.com/go-co-op/gocron/v2"
	"gorm.io/gorm"
)

// Daemon is responsible for scheduling and running monitoring tasks at specified intervals.
// It uses a distributed scheduler to ensure tasks are run reliably across multiple instances.
type Daemon struct {
	scheduler   gocron.Scheduler
	logger      *slog.Logger
	activeTasks map[defs.MonitorTask]*ActiveTask

	started   bool
	startLock sync.Mutex
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
func NewDaemonWithGORMLocker(logger *slog.Logger, db *gorm.DB) (*Daemon, error) {
	workerName, err := randomizer.New().Base64(12)
	if err != nil {
		return nil, fmt.Errorf("failed to generate worker name: %w", err)
	}
	locker, err := gormlock.NewGormLocker(db, workerName, gormlock.WithDefaultJobIdentifier(time.Millisecond))
	if err != nil {
		return nil, fmt.Errorf("failed to create gorm locker: %w", err)
	}

	scheduler, err := gocron.NewScheduler(gocron.WithDistributedLocker(locker))
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	return &Daemon{
		scheduler:   scheduler,
		logger:      logging.Child(logger, "monitor").With(slog.String("worker", workerName)),
		activeTasks: make(map[defs.MonitorTask]*ActiveTask),
	}, nil
}

// Start initializes and begins running the configured monitor tasks according to their schedules.
func (d *Daemon) Start(tasksToStart map[defs.MonitorTask]time.Duration) error {
	d.startLock.Lock()
	defer d.startLock.Unlock()

	if d.started {
		d.logger.Warn("Daemon is already started. Skipping.")
		return nil
	}

	for taskName, taskInterval := range tasksToStart {
		if err := d.initializeTask(taskName, taskInterval); err != nil {
			return err
		}
	}

	d.scheduler.Start()
	d.started = true
	return nil
}

// Get retrieves the active monitoring task associated with the given name.
// Returns the ActiveTask pointer and true if found, otherwise nil and false.
func (d *Daemon) Get(name defs.MonitorTask) (*ActiveTask, bool) {
	task, ok := d.activeTasks[name]
	return task, ok
}

func (d *Daemon) initializeTask(taskName defs.MonitorTask, interval time.Duration) error {
	taskFactory, ok := tasks.All[taskName]
	if !ok {
		d.logger.Warn("Provided unknown task name. Skipping.", slog.Any("task", taskName))
		return nil
	}

	taskInstance := taskFactory(d.logger)

	task := &ActiveTask{
		Instance: taskInstance,
		TaskName: taskName,
		// NOTE: Cronjob (gocron.Job) is not set here, as it will be set when the job is created.
	}

	job, err := d.scheduler.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(d.singleTaskRunner(task)),
		gocron.WithName(fmt.Sprintf("monitor_%s", taskName)),
	)
	if err != nil {
		return fmt.Errorf("failed to create job %s: %w", taskName, err)
	}

	task.Cronjob = job
	d.activeTasks[taskName] = task

	d.logger.Info("Starting a task", slog.Any("task", taskName), slog.Any("interval", interval))
	return nil
}

func (d *Daemon) singleTaskRunner(activeTask *ActiveTask) func() {
	return func() {
		d.logger.Info("Run task", slog.Any("task", activeTask.TaskName))
		defer func() {
			if activeTask.Cronjob == nil {
				return
			}
			nextRun, _ := activeTask.Cronjob.NextRun()
			d.logger.Info("Finish task", slog.Any("task", activeTask.TaskName), slog.Any("next_run", nextRun))
		}()

		activeTask.Instance.Run()
	}
}
