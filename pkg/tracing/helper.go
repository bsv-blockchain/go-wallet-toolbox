package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartTracing starts a new tracing span with the given name and attributes.
func StartTracing(ctx context.Context, spanName string, attributes ...attribute.KeyValue) (context.Context, trace.Span) {
	var span trace.Span
	tracer := otel.Tracer("")
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
