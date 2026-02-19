package certifier

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
)

// Server is a struct for certifier server with the provided wallet and options.
type Server struct {
	wallet     sdk.Interface
	config     *ServerConfig
	service    *CertificateService
	httpServer *http.Server
	addr       string
}

// New creates a new certifier server with the provided wallet and options.
func New(wallet sdk.Interface, opts ...func(*ServerConfig)) (*Server, error) {
	cfg := defaultConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	service := NewCertificateService(wallet, cfg)

	return &Server{
		wallet:  wallet,
		config:  cfg,
		service: service,
	}, nil
}

// Start starts the certifier server. This method blocks until the server is stopped.
func (s *Server) Start() error {
	handler := s.setupRoutes()

	s.addr = ":" + s.config.Port
	s.httpServer = &http.Server{
		Addr:              s.addr,
		Handler:           handler,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      2 * time.Minute,
	}

	s.config.Logger.Info("Listening...", slog.Any("addr", s.addr))

	err := s.httpServer.ListenAndServe()
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

// URL returns the server URL. Only valid after Start() is called.
func (s *Server) URL() string {
	return "http://localhost" + s.addr
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	return s.httpServer.Shutdown(ctx)
}

func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/signCertificate", s.handleSignCertificate)

	authMiddleware := middleware.NewAuth(
		s.wallet,
		middleware.WithAuthLogger(s.config.Logger),
	)

	return authMiddleware.HTTPHandler(mux)
}
