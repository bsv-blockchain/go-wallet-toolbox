package defs

import (
	"fmt"
	"reflect"
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

// Interval returns the monitoring interval as a time.Duration based on IntervalSeconds in the TaskConfig.
func (t *TaskConfig) Interval() time.Duration {
	return time.Duration(must.ConvertToInt64FromUnsigned(t.IntervalSeconds)) * time.Second
}

// TasksConfig is a map of monitoring tasks with their configuration parameters
type TasksConfig struct {
	CheckForProofs TaskConfig `mapstructure:"check_for_proofs"`
}

// All returns a map where each MonitorTask key is paired with its corresponding TaskConfig from the TasksConfig struct.
func (t *TasksConfig) All() map[MonitorTask]TaskConfig {
	result := make(map[MonitorTask]TaskConfig)
	val := reflect.ValueOf(t).Elem()
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := field.Tag.Get("mapstructure")
		if name == "" {
			panic(fmt.Sprintf("missing mapstructure tag for field %s", field.Name))
		}
		if field.Type == reflect.TypeOf(TaskConfig{}) {
			taskName, err := ParseMonitorTaskStr(name)
			if err != nil {
				panic(fmt.Sprintf("invalid task name %s: %v; TaskConfig fields must align with MonitorTask enum type", name, err))
			}
			cfgVal := val.Field(i).Interface().(TaskConfig)
			result[taskName] = cfgVal
		}
	}
	return result
}

// EnabledTasks returns a map of enabled monitoring tasks and their corresponding intervals as time.Duration values.
func (t *TasksConfig) EnabledTasks() map[MonitorTask]time.Duration {
	durations := make(map[MonitorTask]time.Duration)
	for taskName, taskConfig := range t.All() {
		if !taskConfig.Enabled {
			continue
		}
		durations[taskName] = taskConfig.Interval()
	}
	return durations
}

// Validate verifies each task name and configuration in the map, ensuring names are valid and intervals are non-zero.
func (t *TasksConfig) Validate() error {
	for taskName, taskConfig := range t.All() {
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
			CheckForProofs: TaskConfig{
				Enabled:         true,
				IntervalSeconds: 60, // TODO: Will probably need to be extended - for now, it's better to have a short interval for debugging purposes
			},
		},
	}
}
