package defs

import (
	"github.com/go-softwarelab/common/pkg/must"
	"time"
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

// Monitor represents a monitoring system configuration with tasks
type Monitor struct {
	Enabled bool `mapstructure:"enabled"`
	Tasks   TasksConfig
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
