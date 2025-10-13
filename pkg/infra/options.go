package infra

import (
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// Options is the parameters for initializing the "infra" server
type Options struct {
	EnvPrefix  string
	ConfigFile string
	Logger     *slog.Logger
	Tracer     trace.Tracer
}

func defaultOptions() Options {
	return Options{
		EnvPrefix:  "INFRA",
		ConfigFile: "",
		Logger:     nil,
		Tracer:     nil,
	}
}

// InitOption sets a parameter for initializing the "infra" server
type InitOption func(*Options)

// WithEnvPrefix sets the environment variable prefix for the "infra" server, all environment variables will be prefixed with this:
// e.g. "INFRA_HTTP_PORT=8100"
func WithEnvPrefix(prefix string) InitOption {
	return func(o *Options) {
		o.EnvPrefix = prefix
	}
}

// WithConfigFile sets the configuration file for the "infra" server, the configuration file is in YAML format
func WithConfigFile(file string) InitOption {
	return func(o *Options) {
		o.ConfigFile = file
	}
}

// WithLogger sets the logger for the "infra" server
func WithLogger(logger *slog.Logger) InitOption {
	return func(o *Options) {
		o.Logger = logger
	}
}

// WithTracer sets the OpenTelemetry tracer for the infra server
func WithTracer(tracer trace.Tracer) InitOption {
	return func(o *Options) {
		o.Tracer = tracer
	}
}
