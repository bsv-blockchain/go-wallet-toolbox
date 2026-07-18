package defs

import "fmt"

// MetricsConfig configures OpenTelemetry metrics export. Metrics ride the same
// OTLP endpoint configured by TracingConfig.DialAddr — tracing.enabled gates
// span export only, not metrics.
type MetricsConfig struct {
	Enabled               bool `mapstructure:"enabled"`
	ExportIntervalSeconds uint `mapstructure:"export_interval_seconds"`
}

// Observability groups whole-system telemetry configuration. The wallet ships
// no in-process alerting: thresholds and paging live in external tooling.
type Observability struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
}

// DefaultObservability returns the default observability configuration
// (metrics disabled).
func DefaultObservability() Observability {
	return Observability{
		Metrics: MetricsConfig{
			Enabled:               false,
			ExportIntervalSeconds: 15,
		},
	}
}

// Validate checks the observability configuration.
func (o *Observability) Validate() error {
	if o.Metrics.Enabled && o.Metrics.ExportIntervalSeconds == 0 {
		return fmt.Errorf("metrics.export_interval_seconds must be greater than 0 when metrics are enabled")
	}
	return nil
}
