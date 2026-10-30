package telemetry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/telemetry"
)

func TestMeterProvider(t *testing.T) {
	hostProvider := sdkmetric.NewMeterProvider()

	tests := []struct {
		name string
		set  metric.MeterProvider
		want func() metric.MeterProvider
	}{
		{
			name: "unset falls back to the global provider",
			set:  nil,
			want: otel.GetMeterProvider,
		},
		{
			name: "set provider is returned",
			set:  hostProvider,
			want: func() metric.MeterProvider { return hostProvider },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { telemetry.SetMeterProvider(nil) })

			telemetry.SetMeterProvider(tt.set)

			assert.Same(t, tt.want(), telemetry.MeterProvider())
		})
	}
}

func TestTracerProvider(t *testing.T) {
	hostProvider := sdktrace.NewTracerProvider()

	tests := []struct {
		name string
		set  trace.TracerProvider
		want func() trace.TracerProvider
	}{
		{
			name: "unset falls back to the global provider",
			set:  nil,
			want: otel.GetTracerProvider,
		},
		{
			name: "set provider is returned",
			set:  hostProvider,
			want: func() trace.TracerProvider { return hostProvider },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { telemetry.SetTracerProvider(nil) })

			telemetry.SetTracerProvider(tt.set)

			assert.Same(t, tt.want(), telemetry.TracerProvider())
		})
	}
}

func TestSetProvidersLeavesGlobalsUntouched(t *testing.T) {
	globalMeter, globalTracer := otel.GetMeterProvider(), otel.GetTracerProvider()
	t.Cleanup(func() {
		telemetry.SetMeterProvider(nil)
		telemetry.SetTracerProvider(nil)
	})

	telemetry.SetMeterProvider(sdkmetric.NewMeterProvider())
	telemetry.SetTracerProvider(sdktrace.NewTracerProvider())

	assert.Same(t, globalMeter, otel.GetMeterProvider())
	assert.Same(t, globalTracer, otel.GetTracerProvider())
}
