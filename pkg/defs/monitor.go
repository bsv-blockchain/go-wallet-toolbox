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
type EventConfig struct {
	Enabled     bool `mapstructure:"enabled"`
	ChannelSize uint `mapstructure:"channel_size"`
}

// EventsConfig is a struct that contains fields each possible monitoring event
type EventsConfig struct {
	TxBroadcasted EventConfig `mapstructure:"tx_broadcasted"`
	TxProven      EventConfig `mapstructure:"tx_proven"`
}

// Monitor represents a monitoring system configuration with tasks
type Monitor struct {
	Enabled bool         `mapstructure:"enabled"`
	Tasks   TasksConfig  `mapstructure:"tasks"`
	Events  EventsConfig `mapstructure:"events"`
}

// Validate verifies the monitor configuration, including its tasks.
func (m *Monitor) Validate() error {
	return m.Tasks.Validate()
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
	}
}
