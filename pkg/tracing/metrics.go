package tracing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// EnableMetrics sets up OpenTelemetry metrics export over the same OTLP
// endpoint the tracing configuration uses (tracing.enabled gates span export
// only, not metrics). Instruments registered anywhere in the process via
// otel.Meter become live once this runs; without it they are no-ops.
func EnableMetrics(logger *slog.Logger, serviceName, dialAddr string, exportInterval time.Duration) (func(), error) {
	if dialAddr == "" {
		return nil, errors.New("metrics enabled, but OTLP address (tracing.dialAddr) empty")
	}

	ctx := context.Background()

	exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(dialAddr), otlpmetricgrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, fmt.Errorf("failed to create metric resource: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(exportInterval))),
	)
	otel.SetMeterProvider(provider)

	cleanup := func() {
		if shutdownErr := provider.Shutdown(ctx); shutdownErr != nil {
			logger.ErrorContext(ctx, "Failed to shutdown meter provider", slog.String("err", shutdownErr.Error()))
		}
	}

	return cleanup, nil
}
