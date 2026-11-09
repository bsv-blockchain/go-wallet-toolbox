package tracing_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
)

func TestEnableMetrics(t *testing.T) {
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
		{
			name:     "resource attributes from the environment",
			dialAddr: "http://localhost:4317",
			env:      map[string]string{"OTEL_SERVICE_NAME": "host", "OTEL_RESOURCE_ATTRIBUTES": "service.version=1.2.3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := otel.GetMeterProvider()
			t.Cleanup(func() { otel.SetMeterProvider(prev) })
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			// when: the OTLP exporter dials lazily, so no collector is needed
			cleanup, err := tracing.EnableMetrics(logging.NewTestLogger(t), "test", tt.dialAddr, time.Minute)

			// then: resource construction must not fail (e.g. on conflicting schema URLs)
			require.NoError(t, err)
			require.NotNil(t, cleanup)
			// cleanup is not called: shutdown would try to flush to the absent collector.
		})
	}
}
