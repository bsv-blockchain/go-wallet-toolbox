package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// EnableMetrics sets up global OpenTelemetry metrics export for a standalone process, on the same endpoint
// rules as Enable; embedding hosts use the telemetry package instead. Without either, toolbox instruments are no-ops.
func EnableMetrics(logger *slog.Logger, serviceName, dialAddr string, exportInterval time.Duration) (func(), error) {
	ctx := context.Background()

	var opts []otlpmetricgrpc.Option
	if dialAddr != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpointURL(dialAddr))
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	res, err := newResource(ctx, serviceName)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, err
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
