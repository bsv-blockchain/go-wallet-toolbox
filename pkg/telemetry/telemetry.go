// Package telemetry supplies the OpenTelemetry providers the toolbox records metrics and spans on, so a host
// service that embeds the toolbox can pass its own instead of tracing.Enable/EnableMetrics replacing its globals.
// Set them before creating any toolbox component; unset providers fall back to the OpenTelemetry globals.
package telemetry

import (
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// providers replaces the otel globals the toolbox recorded on directly, scoped to the toolbox.
var providers struct {
	mu     sync.RWMutex
	meter  metric.MeterProvider
	tracer trace.TracerProvider
}

// SetMeterProvider sets mp as the provider for every toolbox instrument created afterwards; nil restores the
// global fallback. Instruments created before the call keep their provider.
func SetMeterProvider(mp metric.MeterProvider) {
	providers.mu.Lock()
	defer providers.mu.Unlock()
	providers.meter = mp
}

// MeterProvider returns the provider set by SetMeterProvider, or otel.GetMeterProvider when none is set.
func MeterProvider() metric.MeterProvider {
	providers.mu.RLock()
	defer providers.mu.RUnlock()
	if providers.meter != nil {
		return providers.meter
	}
	return otel.GetMeterProvider()
}

// SetTracerProvider sets tp as the provider for toolbox spans started afterwards; nil restores the global
// fallback.
func SetTracerProvider(tp trace.TracerProvider) {
	providers.mu.Lock()
	defer providers.mu.Unlock()
	providers.tracer = tp
}

// TracerProvider returns the provider set by SetTracerProvider, or otel.GetTracerProvider when none is set.
func TracerProvider() trace.TracerProvider {
	providers.mu.RLock()
	defer providers.mu.RUnlock()
	if providers.tracer != nil {
		return providers.tracer
	}
	return otel.GetTracerProvider()
}
