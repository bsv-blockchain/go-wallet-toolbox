package defs

import (
	"fmt"
	"iter"
	"reflect"
	"time"

	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq2"
)

// MonitorTask represents a monitoring task type
type MonitorTask string

const (
	// CheckForProofsMonitorTask is a monitoring task that checks for proofs in the wallet.
	CheckForProofsMonitorTask MonitorTask = "check_for_proofs"

	// SendWaitingMonitorTask is a monitoring task that check for transactions that have not been sent yet and try to broadcast them.
	SendWaitingMonitorTask MonitorTask = "send_waiting"

	// FailAbandonedMonitorTask marks transactions as failed if they have been abandoned or not processed within a set period.
	FailAbandonedMonitorTask MonitorTask = "fail_abandoned"

	// UnFailMonitorTask is a monitoring task that checks for failed transactions and reverifies failed tx statuses.
	UnFailMonitorTask MonitorTask = "un_fail"
)

// ParseMonitorTaskStr parses a string to a MonitorTask or returns an error
func ParseMonitorTaskStr(task string) (MonitorTask, error) {
	return parseEnumCaseInsensitive(task, CheckForProofsMonitorTask, SendWaitingMonitorTask, FailAbandonedMonitorTask, UnFailMonitorTask)
}

// TaskConfig defines configuration parameters for a monitoring task
type TaskConfig struct {
	Enabled          bool `mapstructure:"enabled"`
	IntervalSeconds  uint `mapstructure:"interval_seconds"`
	StartImmediately bool `mapstructure:"start_immediately"`
}

// Interval returns the monitoring interval as a time.Duration based on IntervalSeconds in the TaskConfig.
func (t *TaskConfig) Interval() time.Duration {
	return time.Duration(must.ConvertToInt64FromUnsigned(t.IntervalSeconds)) * time.Second
}

// TasksConfig is a map of monitoring tasks with their configuration parameters
// This is a struct that contains fields for each MonitorTask
// Make sure each field has a mapstructure tag that matches the MonitorTask enum value.
type TasksConfig struct {
	CheckForProofs TaskConfig `mapstructure:"check_for_proofs"`
	SendWaiting    TaskConfig `mapstructure:"send_waiting"`
	FailAbandoned  TaskConfig `mapstructure:"fail_abandoned"`
	UnFail         TaskConfig `mapstructure:"un_fail"`
}

func (t *TasksConfig) all() iter.Seq2[MonitorTask, TaskConfig] {
	if t == nil {
		return seq2.Empty[MonitorTask, TaskConfig]()
	}

	return func(yield func(MonitorTask, TaskConfig) bool) {
		val := reflect.ValueOf(t).Elem()
		typ := val.Type()
		taskConfigType := reflect.TypeOf(TaskConfig{})

		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			name := field.Tag.Get("mapstructure")
			if name == "" {
				panic(fmt.Sprintf("missing mapstructure tag for field %s", field.Name))
			}
			if field.Type != taskConfigType {
				continue
			}
			taskName, err := ParseMonitorTaskStr(name)
			if err != nil {
				panic(fmt.Sprintf("invalid task name %s: %v; TaskConfig fields must align with MonitorTask enum type", name, err))
			}
			cfgVal := val.Field(i).Interface().(TaskConfig)
			if !yield(taskName, cfgVal) {
				break
			}
		}
	}
}

// EnabledTasks returns a map of enabled monitoring tasks and their corresponding intervals as time.Duration values.
func (t *TasksConfig) EnabledTasks() map[MonitorTask]TaskConfig {
	tasks := make(map[MonitorTask]TaskConfig)
	for taskName, taskConfig := range t.all() {
		if !taskConfig.Enabled {
			continue
		}
		tasks[taskName] = taskConfig
	}
	return tasks
}

// Validate verifies each task name and configuration in the map, ensuring names are valid and intervals are non-zero.
func (t *TasksConfig) Validate() error {
	for taskName, taskConfig := range t.all() {
		if taskConfig.IntervalSeconds == 0 {
			return fmt.Errorf("task %s has interval_seconds set to 0", taskName)
		}
	}
	return nil
}

// EventConfig defines configuration parameters for monitoring events
// If enabled is true, the event will be emitted with the specified channel size.
// The channel size does not limit how many events are delivered: events the channel
// cannot take yet are buffered in memory, so a slow reader never misses an event.
type EventConfig struct {
	Enabled     bool `mapstructure:"enabled"`
	ChannelSize uint `mapstructure:"channel_size"`
}

// EventsConfig is a struct that contains fields each possible monitoring event
type EventsConfig struct {
	TxBroadcasted EventConfig `mapstructure:"tx_broadcasted"`
	TxProven      EventConfig `mapstructure:"tx_proven"`
}

// ReorgMode selects how the monitor reacts to chain reorganization events.
type ReorgMode string

const (
	// ReorgModeImmediate invalidates merkle proofs of orphaned blocks as soon as the reorg
	// is reported; the check_for_proofs task then re-proves the affected transactions.
	ReorgModeImmediate ReorgMode = "immediate"

	// ReorgModeDelayed queues orphaned block hashes and re-proves their transactions only
	// after AgeSeconds, retrying up to MaxRetries times (TypeScript TaskReorg behaviour).
	ReorgModeDelayed ReorgMode = "delayed"
)

// ParseReorgMode parses a string to a ReorgMode or returns an error
func ParseReorgMode(mode string) (ReorgMode, error) {
	return parseEnumCaseInsensitive(mode, ReorgModeImmediate, ReorgModeDelayed)
}

// Reorg configures how the monitor handles chain reorganizations.
type Reorg struct {
	Mode ReorgMode `mapstructure:"mode"`

	// AgeSeconds is how long an orphaned block hash waits before its transactions are
	// re-proved, and again before each retry. Used only in delayed mode.
	AgeSeconds uint `mapstructure:"age_seconds"`

	// MaxRetries is how many re-prove attempts are made for an orphaned block before giving
	// up and keeping the original proof. Used only in delayed mode.
	MaxRetries uint `mapstructure:"max_retries"`

	// MaxQueuedHeaders bounds the delayed-mode queue; the oldest entry is evicted when full.
	MaxQueuedHeaders uint `mapstructure:"max_queued_headers"`
}

// Age returns AgeSeconds as a time.Duration.
func (r *Reorg) Age() time.Duration {
	return time.Duration(must.ConvertToInt64FromUnsigned(r.AgeSeconds)) * time.Second
}

// Validate verifies the reorg configuration.
func (r *Reorg) Validate() error {
	mode, err := ParseReorgMode(string(r.Mode))
	if err != nil {
		return fmt.Errorf("invalid reorg mode: %w", err)
	}

	if mode != ReorgModeDelayed {
		return nil
	}

	if r.AgeSeconds == 0 || r.MaxRetries == 0 || r.MaxQueuedHeaders == 0 {
		return fmt.Errorf("reorg mode %q requires age_seconds, max_retries and max_queued_headers to be greater than 0", mode)
	}
	return nil
}

// DefaultReorg returns the default reorg configuration (immediate mode).
func DefaultReorg() Reorg {
	return Reorg{
		Mode:             ReorgModeImmediate,
		AgeSeconds:       must.ConvertToUInt((10 * time.Minute).Seconds()),
		MaxRetries:       3,
		MaxQueuedHeaders: 4096,
	}
}

// Monitor represents a monitoring system configuration with tasks
type Monitor struct {
	Enabled bool         `mapstructure:"enabled"`
	Tasks   TasksConfig  `mapstructure:"tasks"`
	Events  EventsConfig `mapstructure:"events"`
	Reorg   Reorg        `mapstructure:"reorg"`
}

// Validate verifies the monitor configuration, including its tasks.
func (m *Monitor) Validate() error {
	if err := m.Tasks.Validate(); err != nil {
		return err
	}
	return m.Reorg.Validate()
}

// DefaultMonitorConfig returns a default monitoring configuration
func DefaultMonitorConfig() Monitor {
	return Monitor{
		Enabled: true,
		Tasks: TasksConfig{
			CheckForProofs: TaskConfig{
				Enabled:         true,
				IntervalSeconds: must.ConvertToUInt((1 * time.Minute).Seconds()),
			},
			SendWaiting: TaskConfig{
				// NOTE: Normally, background broadcaster should handle new transactions.
				// NOTE: This task can be considered as a fallback if there are still waiting transactions.
				// NOTE: StartImmediately is set to true to try broadcasting transactions that were in the queue when the app shut down.
				Enabled:          true,
				IntervalSeconds:  must.ConvertToUInt((5 * time.Minute).Seconds()),
				StartImmediately: true,
			},
			FailAbandoned: TaskConfig{
				Enabled:         true,
				IntervalSeconds: must.ConvertToUInt((5 * time.Minute).Seconds()),
			},
			UnFail: TaskConfig{
				Enabled:         true,
				IntervalSeconds: must.ConvertToUInt((10 * time.Minute).Seconds()),
			},
		},
		Events: EventsConfig{
			// Note: Disabled by default because it requires event listeners to be registered to avoid blocking.
			TxBroadcasted: EventConfig{
				Enabled:     false,
				ChannelSize: 100,
			},
			TxProven: EventConfig{
				Enabled:     false,
				ChannelSize: 100,
			},
		},
		Reorg: DefaultReorg(),
	}
}
