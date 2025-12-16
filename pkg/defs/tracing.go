package defs

import "fmt"

// TracingConfig holds the configuration for tracing
type TracingConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	DialAddr string `mapstructure:"dialAddr"`
	Sample   int    `mapstructure:"sample"` // percentage of requests to sample (0-100)
}

// Validate checks if the TracingConfig is valid
func (c *TracingConfig) Validate() (err error) {
	if !c.Enabled {
		return nil
	}

	if c.DialAddr == "" {
		return fmt.Errorf("DialAddr for tracing is required")
	}

	return nil
}

// DefaultTracingConfig returns the default tracing configuration
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:  false,
		DialAddr: "http://localhost:4317",
		Sample:   100,
	}
}
