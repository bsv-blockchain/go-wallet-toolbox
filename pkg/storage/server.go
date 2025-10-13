package storage

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/server"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Server represents the storage server exposing JSON-RPC API
type Server struct {
	provider   wdk.WalletStorageProvider
	logger     *slog.Logger
	options    ServerOptions
	wallet     sdk.Interface
	httpServer *http.Server
	tracer     trace.Tracer
}

// NewServer creates a new storage server instance with given storage provider and optional options
func NewServer(logger *slog.Logger, storage wdk.WalletStorageProvider, wallet sdk.Interface, opts ServerOptions) *Server {
	tracer := opts.Tracer
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("storage.Server")
	}

	return &Server{
		provider: storage,
		wallet:   wallet,
		logger:   logging.Child(logger, "StorageServer"),
		options:  opts,
		tracer:   tracer,
	}
}

// Handler returns an http.Handler configured with the storage RPC endpoints.
func (s *Server) Handler() http.Handler {
	provider := server.NewRPCStorageProvider(s.logger, s.provider)

	rpcServer := server.NewRPCHandler(s.logger, "remote_storage", provider)

	mux := http.NewServeMux()
	rpcServer.Register(mux)

	authMiddleware := middleware.NewAuth(s.wallet, middleware.WithAuthLogger(s.logger))

	// allow the API to be used everywhere when CORS is enforced.
	return server.AllowAllCORSMiddleware(authMiddleware.HTTPHandler(mux))
}

// Start starts the server
// NOTE: This method is blocking
func (s *Server) Start(ctx context.Context) error {
	ctxWithSpan, span := s.tracer.Start(ctx, "storage.Server.Start")
	defer span.End()

	port := s.options.Port
	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           s.Handler(), // Handler is auto-instrumented below
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      2 * time.Minute,
		BaseContext:       func(net.Listener) context.Context { return ctxWithSpan },
	}

	s.logger.Info("Listening...", slog.Any("port", port))

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	s.logger.Info("Shutting down server...", slog.Any("port", s.options.Port))
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("error shutting down storage server: %w", err)
	}
	return nil
}
