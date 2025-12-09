package chaintracks

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
)

// Server coordinates the HTTP server, application handler, logging, and configuration for Chaintracks services.
type Server struct {
	Handler *Handler
	Service *Service

	logger *slog.Logger
	config defs.ChaintracksServerConfig
}

// NewServer creates and returns a new Server instance with the provided logger and configuration.
// Returns an error if the handler cannot be initialized.
func NewServer(logger *slog.Logger, config defs.ChaintracksServerConfig) (*Server, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid chaintracks service config: %w", err)
	}

	logger = logging.Child(logger, "chaintracks_server")

	service, err := NewService(logger, config.ChaintracksServiceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create chaintracks service: %w", err)
	}

	handler, err := NewHandler(logger, service)
	if err != nil {
		return nil, fmt.Errorf("failed to create chaintracks handler: %w", err)
	}

	return &Server{
		Handler: handler,
		Service: service,

		logger: logger,
		config: config,
	}, nil
}

// ListenAndServe starts the server
// NOTE: This method is blocking
func (s *Server) ListenAndServe(ctx context.Context) error {
	s.logger.Info("Making chaintracks service available...")
	if err := s.Service.MakeAvailable(ctx); err != nil {
		return fmt.Errorf("failed to make chaintracks service available: %w", err)
	}
	s.logger.Info("Chaintracks service is now available")

	mainMux := http.NewServeMux()
	prefix := ""
	if s.config.RoutingPrefix != "" {
		prefix = "/" + s.config.RoutingPrefix
	}
	mainMux.Handle(prefix+"/", http.StripPrefix(prefix, s.Handler.Handler()))

	port := s.config.Port
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mainMux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      2 * time.Minute,
	}

	s.logger.Info("Listening...", slog.Any("port", port))
	err := httpServer.ListenAndServe()
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

// MakeLogger creates and returns a logger based on the provided LogConfig settings.
// Returns a no-op logger if logging is disabled in the config.
// Sets log level and handler type according to the config values.
// Adds a "service" attribute with value "chaintracks" to all log entries.
func MakeLogger(config defs.LogConfig) *slog.Logger {
	if !config.Enabled {
		return logging.New().Nop().Logger()
	}

	logger := logging.New().
		WithLevel(config.Level).
		WithHandler(config.Handler, os.Stdout).
		Logger()

	return logging.Child(logger, "chaintracks")
}
