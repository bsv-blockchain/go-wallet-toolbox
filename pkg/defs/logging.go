package defs

import (
	"fmt"
)

// LogLevel represents different log levels which can be configured.
type LogLevel string

// Supported log levels (based on slog).
const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// ParseLogLevelStr parses a string into a LogLevel (case-insensitive).
func ParseLogLevelStr(level string) (LogLevel, error) {
	return parseEnumCaseInsensitive(level, LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError)
}

// LogHandler represents different log handler types which can be configured.
type LogHandler string

// Supported handler types (based on slog).
const (
	JSONHandler LogHandler = "json"
	TextHandler LogHandler = "text"
)

// ParseHandlerTypeStr parses a string into a LogHandler (case-insensitive).
func ParseHandlerTypeStr(handlerType string) (LogHandler, error) {
	return parseEnumCaseInsensitive(handlerType, JSONHandler, TextHandler)
}

// LogConfig is the configuration for the logging
type LogConfig struct {
	Enabled bool       `mapstructure:"enabled"`
	Level   LogLevel   `mapstructure:"level"`
	Handler LogHandler `mapstructure:"handler"`
}

// Validate validates the HTTP configuration
func (c *LogConfig) Validate() (err error) {
	if c.Level, err = ParseLogLevelStr(string(c.Level)); err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	if c.Handler, err = ParseHandlerTypeStr(string(c.Handler)); err != nil {
		return fmt.Errorf("invalid log handler: %w", err)
	}

	return nil
}

// DefaultLogConfig returns a LogConfig with logging enabled, level set to info, and using the JSON handler for output.
func DefaultLogConfig() LogConfig {
	return LogConfig{
		Enabled: true,
		Level:   LogLevelInfo,
		Handler: JSONHandler,
	}
}
