package certifier

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware"
	primitives "github.com/bsv-blockchain/go-sdk/primitives/ec"
	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
)

// Server is a struct for certifier server with the provided wallet and options.
type Server struct {
	wallet     sdk.Interface
	config     *ServerConfig
	service    *CertificateService
	httpServer *http.Server
	addr       string
	cleanup    func()
}

// NewServer creates a new certifier server from a configuration file.
func NewServer(ctx context.Context, configFile string) (*Server, error) {
	loader := config.NewLoader(ConfigDefaults, "CERTIFIER")
	if err := loader.SetConfigFilePath(configFile); err != nil {
		return nil, fmt.Errorf("failed to set config file path: %w", err)
	}

	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err = cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	logger := makeLogger(&cfg)

	certifierWallet, cleanup, err := createCertifierWallet(ctx, &cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create certifier wallet: %w", err)
	}

	server, err := New(certifierWallet,
		WithPort(cfg.Server.Port),
		WithLogger(logger),
		WithRandomizer(randomizer.New()),
		WithOriginator("certifier-server"),
	)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to create certifier server: %w", err)
	}

	server.cleanup = cleanup

	return server, nil
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
	if err != nil && err != http.ErrServerClosed {
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
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}
	if s.cleanup != nil {
		s.cleanup()
	}
	return nil
}

// Handler returns the HTTP handler for the server.
// This is useful for testing with httptest.Server.
func (s *Server) Handler() http.Handler {
	return s.setupRoutes()
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

func createCertifierWallet(_ context.Context, cfg *Config, logger *slog.Logger) (*wallet.Wallet, func(), error) {
	privateKey, err := primitives.PrivateKeyFromHex(cfg.CertifierWallet.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create private key from hex %s: %w", cfg.CertifierWallet.PrivateKey, err)
	}

	// Create proto wallet for storage authentication
	protoWallet, err := sdk.NewCompletedProtoWallet(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create proto wallet: %w", err)
	}

	// Create storage client using proto wallet for auth
	storageClient, storageCleanup, err := storage.NewClient(
		cfg.Storage.URL,
		protoWallet,
		storage.WithClientLogger(logger),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	// Create full wallet with storage client
	certifierWallet, err := wallet.New(
		cfg.Server.Network,
		privateKey,
		storageClient,
		wallet.WithLogger(logger),
	)
	if err != nil {
		storageCleanup()
		return nil, nil, fmt.Errorf("failed to create certifier wallet: %w", err)
	}
	cleanup := func() {
		storageCleanup()
		certifierWallet.Close()
	}
	return certifierWallet, cleanup, nil
}

func makeLogger(cfg *Config) *slog.Logger {
	if !cfg.Logging.Enabled {
		return logging.New().Nop().Logger()
	}
	return logging.New().
		WithLevel(cfg.Logging.Level).
		WithHandler(cfg.Logging.Handler, os.Stdout).
		Logger()
}
