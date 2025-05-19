package defs

import (
	"fmt"
	"time"

	"github.com/go-softwarelab/common/pkg/must"
)

// MonitorTask represents a monitoring task type
type MonitorTask string

const (
	// CheckForProofsMonitorTask is a monitoring task that checks for proofs in the wallet.
	CheckForProofsMonitorTask MonitorTask = "check_for_proofs"
)

// ParseMonitorTaskStr parses a string to a MonitorTask or returns an error
func ParseMonitorTaskStr(task string) (MonitorTask, error) {
	return parseEnumCaseInsensitive(task, CheckForProofsMonitorTask)
}

// TaskConfig defines configuration parameters for a monitoring task
type TaskConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	IntervalSeconds uint `mapstructure:"interval_seconds"`
}

// TasksConfig is a map of monitoring tasks with their configuration parameters
type TasksConfig map[MonitorTask]TaskConfig

// EnabledTasks returns a map of durations for enabled tasks
func (t TasksConfig) EnabledTasks() map[MonitorTask]time.Duration {
	durations := make(map[MonitorTask]time.Duration)
	for taskName, taskConfig := range t {
		durations[taskName] = time.Duration(must.ConvertToInt64FromUnsigned(taskConfig.IntervalSeconds)) * time.Second
	}
	return durations
}

// Validate verifies each task name and configuration in the map, ensuring names are valid and intervals are non-zero.
func (t TasksConfig) Validate() error {
	for taskName, taskConfig := range t {
		sanitizedTaskName, err := ParseMonitorTaskStr(string(taskName))
		if err != nil {
			return fmt.Errorf("task %s is not a valid task name: %w", taskName, err)
		}

		if sanitizedTaskName != taskName {
			t[sanitizedTaskName] = taskConfig
			delete(t, taskName)
		}

		if taskConfig.IntervalSeconds == 0 {
			return fmt.Errorf("task %s has interval_seconds set to 0", taskName)
		}
	}
	return nil
}

// Monitor represents a monitoring system configuration with tasks
type Monitor struct {
	Enabled bool        `mapstructure:"enabled"`
	Tasks   TasksConfig `mapstructure:"tasks"`
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
			CheckForProofsMonitorTask: {
				Enabled:         true,
				IntervalSeconds: 60, // TODO: Will probably need to be extended - for now, it's better to have a short interval for debugging purposes
			},
		},
	}
}
