package tracing_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/telemetry"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
)

func TestStartTracingUsesTelemetryTracerProvider(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	telemetry.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { telemetry.SetTracerProvider(nil) })

	_, span := tracing.StartTracing(t.Context(), "operation")
	tracing.EndTracing(span, errors.New("failed"))

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, "github.com/bsv-blockchain/go-wallet-toolbox", ended[0].InstrumentationScope().Name)
	assert.Equal(t, codes.Error, ended[0].Status().Code)
}

func TestNewTraceProviderResource(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantName    string
		wantVersion string
	}{
		{
			name:     "service name from the caller",
			wantName: "server",
		},
		{
			name:        "service name and version from the environment",
			env:         map[string]string{"OTEL_SERVICE_NAME": "host", "OTEL_RESOURCE_ATTRIBUTES": "service.version=1.2.3"},
			wantName:    "host",
			wantVersion: "1.2.3",
		},
		{
			name:     "malformed resource attributes are ignored",
			env:      map[string]string{"OTEL_RESOURCE_ATTRIBUTES": "malformed"},
			wantName: "server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreGlobals(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			tp, _, err := tracing.NewTraceProvider(t.Context(), "server", 100)
			require.NoError(t, err)

			attrs := spanResource(t, tp).Set()
			name, _ := attrs.Value("service.name")
			assert.Equal(t, tt.wantName, name.AsString())
			version, _ := attrs.Value("service.version")
			assert.Equal(t, tt.wantVersion, version.AsString())
			// the provider is not shut down: shutdown would try to flush to the absent collector.
		})
	}
}

func TestEnable(t *testing.T) {
	tests := []struct {
		name     string
		dialAddr string
		env      map[string]string
	}{
		{
			name:     "explicit dial address",
			dialAddr: "http://localhost:4317",
		},
		{
			name: "endpoint from the standard OTLP environment variable",
			env:  map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreGlobals(t)
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			cleanup, err := tracing.Enable(logging.NewTestLogger(t), "test", tt.dialAddr, 100)

			require.NoError(t, err)
			require.NotNil(t, cleanup)
			// no spans were recorded, so cleanup has nothing to flush to the absent collector
			cleanup()
		})
	}
}

func restoreGlobals(t *testing.T) {
	t.Helper()
	tracerProvider, propagator := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(tracerProvider)
		otel.SetTextMapPropagator(propagator)
	})
}

func spanResource(t *testing.T, tp *sdktrace.TracerProvider) *resource.Resource {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp.RegisterSpanProcessor(recorder)
	_, span := tp.Tracer("test").Start(t.Context(), "operation")
	span.End()

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	return ended[0].Resource()
}
