package storage

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/server"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

// Server represents the storage server exposing JSON-RPC API
type Server struct {
	provider wdk.WalletStorageProvider
	logger   *slog.Logger
	options  ServerOptions
}

// NewServer creates a new storage server instance with given storage provider and optional options
func NewServer(logger *slog.Logger, storage wdk.WalletStorageProvider, opts ServerOptions) *Server {
	return &Server{
		provider: storage,
		logger:   logging.Child(logger, "storage_server"),
		options:  opts,
	}
}

// Handler returns an http.Handler configured with the storage RPC endpoints.
func (s *Server) Handler() http.Handler {
	rpcServer := server.NewRPCHandler(s.logger, "remote_storage", s.provider)

	mux := http.NewServeMux()
	rpcServer.Register(mux)

	return mux
}

// Start starts the server
// NOTE: This method is blocking
func (s *Server) Start() error {
	port := s.options.Port
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           s.Handler(),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	s.logger.Info("Listening...", slog.Any("port", port))
	err := httpServer.ListenAndServe()
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}
