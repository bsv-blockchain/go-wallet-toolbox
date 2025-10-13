package storage

import "go.opentelemetry.io/otel/trace"

// ServerOptions represents configurable options for the storage server
type ServerOptions struct {
	Port   uint
	Tracer trace.Tracer
}
