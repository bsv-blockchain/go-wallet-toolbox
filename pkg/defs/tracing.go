package defs

import "fmt"

type TracingConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	DialAddr string `mapstructure:"dialAddr"`
	Sample   int    `mapstructure:"sample"`
}

func (c *TracingConfig) Validate() (err error) {
	if !c.Enabled {
		return nil
	}

	if c.DialAddr == "" {
		return fmt.Errorf("DialAddr for tracing is required")
	}

	return nil
}

func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:  true,
		DialAddr: "http://localhost:4317",
		Sample:   100,
	}
}
