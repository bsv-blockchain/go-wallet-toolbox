package chaintracks

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
)

// Server coordinates the HTTP server, application handler, logging, and configuration for Chaintracks services.
type Server struct {
	Handler *Handler

	logger *slog.Logger
	config defs.ChaintracksServerConfig
}

// NewServer creates and returns a new Server instance with the provided logger and configuration.
// Returns an error if the handler cannot be initialized.
func NewServer(logger *slog.Logger, config defs.ChaintracksServerConfig) (*Server, error) {
	logger = logging.Child(logger, "chaintracks_server")

	handler, err := NewHandler(logger, config.ChaintracksServiceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create chaintracks handler: %w", err)
	}

	return &Server{
		logger:  logger,
		Handler: handler,
		config:  config,
	}, nil
}

// ListenAndServe starts the server
// NOTE: This method is blocking
func (s *Server) ListenAndServe() error {
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
