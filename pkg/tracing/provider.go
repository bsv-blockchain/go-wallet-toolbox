package tracing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// NewTraceProvider creates a new OpenTelemetry trace provider with the given service name and sampling rate.
func NewTraceProvider(ctx context.Context, serviceName string, sample int, opts ...otlptracegrpc.Option) (*trace.TracerProvider, *otlptrace.Exporter, error) {
	exporter, err := otlptracegrpc.New(
		ctx,
		opts...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	traceOpt := trace.WithSampler(trace.AlwaysSample())
	if sample != 0 && sample != 100 {
		traceOpt = trace.WithSampler(trace.TraceIDRatioBased(float64(sample) / 100))
	}

	tp := trace.NewTracerProvider(
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		)),
		trace.WithBatcher(exporter),
		traceOpt,
	)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetTracerProvider(tp)

	return tp, exporter, nil
}

// Enable sets up OpenTelemetry tracing with the given logger, service name, dial address, and sampling rate.
func Enable(logger *slog.Logger, serviceName, dialAddr string, sample int) (func(), error) {
	if dialAddr == "" {
		return nil, errors.New("tracing enabled, but tracing address empty")
	}

	ctx := context.Background()

	tp, exporter, err := NewTraceProvider(ctx, serviceName, sample, otlptracegrpc.WithEndpointURL(dialAddr), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to create trace provider: %w", err)
	}

	cleanup := func() {
		// Spans are batched, so anything produced in the last few seconds is
		// still queued in the provider. Flush and shut the provider down first:
		// it is the provider that drains the queue through the exporter, so
		// closing the exporter first silently discards everything still pending.
		//
		// Long-running services never noticed - the batch timer had already
		// exported their spans - but a short-lived process would finish its work
		// and lose all of it on the way out.
		if err = tp.ForceFlush(ctx); err != nil {
			logger.ErrorContext(ctx, "Failed to flush pending spans", slog.String("err", err.Error()))
		}

		if err = tp.Shutdown(ctx); err != nil {
			logger.ErrorContext(ctx, "Failed to shutdown tracing provider", slog.String("err", err.Error()))
		}

		if err = exporter.Shutdown(ctx); err != nil {
			logger.ErrorContext(ctx, "Failed to shutdown exporter", slog.String("err", err.Error()))
		}
	}

	return cleanup, nil
}
