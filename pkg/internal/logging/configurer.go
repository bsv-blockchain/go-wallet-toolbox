package logging

import (
	"io"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
)

// Configurer is the main interface for configuring a logger.
type Configurer interface {
	WithLevel(level defs.LogLevel) HandlerConfigurer
	Nop() LoggerMaker
}

// HandlerConfigurer is an interface for configuring a logger handler.
type HandlerConfigurer interface {
	WithCustomHandler(handler slog.Handler) LoggerMaker
	WithHandler(handlerType defs.LogHandler, writer io.Writer) LoggerMaker
}

// LoggerMaker is an interface for creating a logger from a ready configuration.
type LoggerMaker interface {
	Logger() *slog.Logger
}

type configurer struct {
	handler slog.Handler
	level   *slog.LevelVar
}

// New creates a new Configurer for configuring a logger.
func New() Configurer {
	return &configurer{
		level: new(slog.LevelVar),
	}
}

// WithLevel sets the log level for the logger.
func (c *configurer) WithLevel(level defs.LogLevel) HandlerConfigurer {
	c.level.Set(strLevelToSlog[level])
	return c
}

// Nop sets the special logger  handler to discard all logs.
func (c *configurer) Nop() LoggerMaker {
	c.handler = slog.DiscardHandler
	return c
}

// WithCustomHandler sets a custom handler for the logger.
func (c *configurer) WithCustomHandler(handler slog.Handler) LoggerMaker {
	c.handler = handler
	return c
}

// WithHandler sets a handler for the logger (provided by slog package).
func (c *configurer) WithHandler(handlerType defs.LogHandler, writer io.Writer) LoggerMaker {
	opts := &slog.HandlerOptions{Level: c.level}

	switch handlerType {
	case defs.JSONHandler:
		c.handler = slog.NewJSONHandler(writer, opts)
	case defs.TextHandler:
		c.handler = slog.NewTextHandler(writer, opts)
	default:
		panic("unsupported handler type")
	}
	return c
}

// Logger creates a new logger from the configuration.
func (c *configurer) Logger() *slog.Logger {
	return slog.New(c.handler)
}
