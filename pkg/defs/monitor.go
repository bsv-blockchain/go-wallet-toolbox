package defs

// MonitorTask represents a monitoring task type
type MonitorTask string

const (
	// TimeMonitorTask is a monitoring task for time-based operations
	TimeMonitorTask MonitorTask = "time"
)

// ParseMonitorTaskStr parses a string to a MonitorTask or returns an error
func ParseMonitorTaskStr(task string) (MonitorTask, error) {
	return parseEnumCaseInsensitive(task, TimeMonitorTask)
}

// TaskConfig defines configuration parameters for a monitoring task
type TaskConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	IntervalSeconds uint `mapstructure:"interval_seconds"`
}

// Monitor represents a monitoring system configuration with tasks
type Monitor struct {
	Enabled bool `mapstructure:"enabled"`
	Tasks   map[MonitorTask]TaskConfig
}

// DefaultMonitorConfig returns a default monitoring configuration
func DefaultMonitorConfig() Monitor {
	return Monitor{
		Enabled: true,
		Tasks:   map[MonitorTask]TaskConfig{
			TimeMonitorTask: {
				Enabled:         true,
				IntervalSeconds: 1,
			},
		},
	}
}
