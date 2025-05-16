package monitor

import (
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/monitor/internal/tasks"
	gormlock "github.com/go-co-op/gocron-gorm-lock/v2"
	"github.com/go-co-op/gocron/v2"
	"github.com/go-softwarelab/common/pkg/must"
	"gorm.io/gorm"
	"log/slog"
	"time"
)

// Daemon is responsible for scheduling and running monitoring tasks at specified intervals.
// It uses a distributed scheduler to ensure tasks are run reliably across multiple instances.
type Daemon struct {
	scheduler   gocron.Scheduler
	logger      *slog.Logger
	activeTasks map[defs.MonitorTask]tasks.TaskInterface
}

// NewDaemonWithGORMLocker creates a new Daemon instance with a GORM-based distributed lock.
// This ensures that scheduled tasks run on only one instance when multiple application instances are deployed.
func NewDaemonWithGORMLocker(logger *slog.Logger, db *gorm.DB) (*Daemon, error) {
	workerName, err := randomizer.New().Base64(12)
	if err != nil {
		return nil, fmt.Errorf("failed to generate worker name: %w", err)
	}
	locker, err := gormlock.NewGormLocker(db, workerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create gorm locker: %w", err)
	}

	scheduler, err := gocron.NewScheduler(gocron.WithDistributedLocker(locker))
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	return &Daemon{
		scheduler: scheduler,
		logger:    logging.Child(logger, "monitor"),
		activeTasks: make(map[defs.MonitorTask]tasks.TaskInterface),
	}, nil
}

// Start initializes and begins running the configured monitor tasks according to their schedules.
func (d *Daemon) Start(monitor map[defs.MonitorTask]defs.TaskConfig) error {
	for taskName, taskConfig := range monitor {
		if err := d.initializeTask(taskName, taskConfig); err != nil {
			return err
		}
	}

	d.scheduler.Start()
	return nil
}

// initializeTask initializes and schedules a single monitoring task.
func (d *Daemon) initializeTask(taskName defs.MonitorTask, taskConfig defs.TaskConfig) error {
	taskCreator, ok := tasks.All[taskName]
	if !ok {
		d.logger.Warn("Provided unknown task name. Skipping.", slog.Any("task", taskName))
		return nil
	}

	if !taskConfig.Enabled {
		d.logger.Debug("Task is disabled", slog.Any("task", taskName))
		return nil
	}

	taskInstance := taskCreator(d.logger)
	d.activeTasks[taskName] = taskInstance

	intervalDuration := time.Duration(must.ConvertToInt64FromUnsigned(taskConfig.IntervalSeconds)) * time.Second
	_, err := d.scheduler.NewJob(
		gocron.DurationJob(intervalDuration),
		gocron.NewTask(func() { taskInstance.Run() }),
		gocron.WithName(fmt.Sprintf("monitor_%s", taskName)),
	)
	if err != nil {
		return fmt.Errorf("failed to create job %s: %w", taskName, err)
	}

	d.logger.Info("Starting a task", slog.Any("task", taskName), slog.Any("interval", intervalDuration))
	return nil
}
