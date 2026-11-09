package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/telemetry"
)

const tracerName = "github.com/bsv-blockchain/go-wallet-toolbox"

// StartTracing starts a new tracing span with the given name and attributes on telemetry.TracerProvider.
//
//nolint:spancheck // span is returned to caller who is responsible for ending it via EndTracing
func StartTracing(ctx context.Context, spanName string, attributes ...attribute.KeyValue) (context.Context, trace.Span) {
	var span trace.Span
	tracer := telemetry.TracerProvider().Tracer(tracerName)
	if tracer == nil {
		return ctx, nil
	}

	ctx, span = tracer.Start(ctx, spanName, trace.WithAttributes(attributes...))
	return ctx, span
}

// EndTracing ends the given tracing span, recording any error if present.
func EndTracing(span trace.Span, err error) {
	if span != nil {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}
