package storage

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	servercommon "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/server"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/server"
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

// Handler returns an http.Handler configured with the storage RPC endpoints.
func (s *Server) Handler() http.Handler {
	provider := server.NewRPCStorageProvider(s.logger, s.provider)

	rpcServer := server.NewRPCHandler(s.logger, "remote_storage", provider)

	mux := http.NewServeMux()
	rpcServer.Register(mux)

	var handler http.Handler = mux

	if s.options.Monetize {
		paymentMiddleware := middleware.NewPayment(s.wallet, withOptionalRequestPriceCalculator(s.options.CalculateRequestPrice), middleware.WithPaymentLogger(s.logger))
		handler = paymentMiddleware.HTTPHandler(handler)
	} else {
		s.logger.Info("Payment middleware is disabled (Monetize=false)")
		if s.options.CalculateRequestPrice != nil {
			s.logger.Warn("CalculateRequestPrice option is set but will be ignored because Monetize=false")
		}
	}

	authMiddleware := middleware.NewAuth(s.wallet, middleware.WithAuthLogger(s.logger))
	handler = authMiddleware.HTTPHandler(handler)
	// allow the API to be used everywhere when CORS is enforced.
	handler = servercommon.AllowAllCORSMiddleware(handler)

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

	s.logger.Info("Listening...", slog.Any("port", port))
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
