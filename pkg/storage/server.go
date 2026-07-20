package storage

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	servercommon "github.com/bsv-blockchain/go-wallet-toolbox/pkg/server"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/v2adapter"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Server represents the storage server exposing JSON-RPC API
type Server struct {
	provider wdk.WalletStorageProvider
	logger   *slog.Logger
	options  ServerOptions
	wallet   sdk.Interface
}

// NewServer creates a new storage server instance with given storage provider and optional options
func NewServer(logger *slog.Logger, storage wdk.WalletStorageProvider, wallet sdk.Interface, opts ServerOptions) *Server {
	return &Server{
		provider: storage,
		wallet:   wallet,
		logger:   logging.Child(logger, "StorageServer"),
		options:  opts,
	}
}

// Handler returns an http.Handler configured with the storage v1 adapter endpoints
// (the canonical BRC-100 /storage/v2/* remoting contract).
func (s *Server) Handler() http.Handler {
	// Use the new v2adapter as the core remoting implementation.
	// This replaces the previous JSON-RPC layer.
	coreHandler := v2adapter.NewHandler(s.provider, s.logger)

	handler := coreHandler

	if s.options.Monetize {
		paymentMiddleware := middleware.NewPayment(s.wallet, withOptionalRequestPriceCalculator(s.options.CalculateRequestPrice), middleware.WithPaymentLogger(s.logger))
		handler = paymentMiddleware.HTTPHandler(handler)
	} else {
		s.logger.InfoContext(context.Background(), "Payment middleware is disabled (Monetize=false)")
		if s.options.CalculateRequestPrice != nil {
			s.logger.WarnContext(context.Background(), "CalculateRequestPrice option is set but will be ignored because Monetize=false")
		}
	}

	authOpts := []func(*middleware.AuthMiddlewareConfig){middleware.WithAuthLogger(s.logger)}
	if s.options.AllowUnauthenticated {
		authOpts = append(authOpts, middleware.WithAuthAllowUnauthenticated())
	}
	authMiddleware := middleware.NewAuth(s.wallet, authOpts...)
	handler = authMiddleware.HTTPHandler(handler)
	handler = servercommon.MaxBytesMiddleware(handler, s.maxRequestBodyBytes())
	if corsConfig, ok := s.corsConfig(); ok {
		handler = servercommon.NewCORSMiddleware(handler, corsConfig)
	}

	return handler
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
		WriteTimeout:      2 * time.Minute,
	}

	s.logger.InfoContext(context.Background(), "Listening...", slog.Any("port", port))
	err := httpServer.ListenAndServe()
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

func withOptionalRequestPriceCalculator(calculator func(r *http.Request) (int, error)) func(*middleware.PaymentMiddlewareConfig) {
	if calculator == nil {
		return func(c *middleware.PaymentMiddlewareConfig) {}
	}
	return middleware.WithRequestPriceCalculator(calculator)
}

func (s *Server) maxRequestBodyBytes() int64 {
	if s.options.MaxRequestBodyBytes > 0 {
		return s.options.MaxRequestBodyBytes
	}
	return servercommon.DefaultMaxRequestBodyBytes
}

func (s *Server) corsConfig() (servercommon.CORSConfig, bool) {
	if s.options.DisableCORS {
		return servercommon.CORSConfig{}, false
	}
	config := s.options.CORS
	if isZeroCORSConfig(config) {
		config = DefaultCORSConfig()
	}
	return config, config.Enabled
}

func isZeroCORSConfig(config servercommon.CORSConfig) bool {
	return !config.Enabled &&
		!config.AllowAllOrigins &&
		!config.AllowPrivateNetwork &&
		len(config.AllowedOrigins) == 0 &&
		len(config.AllowedMethods) == 0 &&
		len(config.AllowedHeaders) == 0 &&
		len(config.ExposedHeaders) == 0
}
