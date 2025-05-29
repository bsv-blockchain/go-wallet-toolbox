package infra

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/4chain-ag/go-wallet-toolbox/internal/config"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/monitor"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/services"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	exampleWallet "github.com/bsv-blockchain/go-bsv-middleware-examples/example-wallet"
	"github.com/bsv-blockchain/go-bsv-middleware/pkg/middleware/auth"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
)

// Server is a struct that holds the "infra" server configuration
type Server struct {
	Config Config

	logger        *slog.Logger
	storage       *storage.Provider
	storageServer *storage.Server
	monitor       *monitor.Daemon
}

// NewServer creates a new server instance with given options, like config file path or a prefix for environment variables
func NewServer(ctx context.Context, opts ...InitOption) (*Server, error) {
	options := defaultOptions()
	for _, option := range opts {
		option(&options)
	}

	loader := config.NewLoader(Defaults, options.EnvPrefix)
	if options.ConfigFile != "" {
		err := loader.SetConfigFilePath(options.ConfigFile)
		if err != nil {
			return nil, fmt.Errorf("failed to set config file path: %w", err)
		}
	}
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	err = cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	logger := logging.Child(makeLogger(&cfg, &options), "infra")

	activeServices := services.New(logger, cfg.Services)

	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}

	activeStorage, err := storage.NewGORMProvider(logger, storage.GORMProviderConfig{
		DB:                    cfg.DBConfig,
		Chain:                 cfg.BSVNetwork,
		FeeModel:              cfg.FeeModel,
		Commission:            cfg.Commission,
		Services:              activeServices,
		SynchronizeTxStatuses: cfg.SynchronizeTxStatuses,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}

	_, err = activeStorage.Migrate(ctx, cfg.Name, storageIdentityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate storage: %w", err)
	}

	var daemon *monitor.Daemon
	if cfg.Monitor.Enabled {
		daemon, err = monitor.NewDaemonWithGORMLocker(ctx, logger, activeStorage, activeStorage.Database.DB)
		if err != nil {
			return nil, fmt.Errorf("failed to create daemon: %w", err)
		}

		if err = daemon.Start(cfg.Monitor.Tasks.EnabledTasks()); err != nil {
			return nil, fmt.Errorf("failed to start storage monitor: %w", err)
		}
	}

	sPrivKey, err := ec.PrivateKeyFromHex(cfg.ServerPrivateKey)
	if err != nil {
		log.Fatalf("Failed to create server private key: %v", err)
	}

	serverWallet, err := exampleWallet.NewExtendedProtoWallet(sPrivKey)
	if err != nil {
		log.Fatalf("Failed to create server wallet: %v", err)
	}

	authMiddleware, err := auth.New(auth.Config{
		AllowUnauthenticated: false, // or true based on your needs
		Logger:               logger,
		Wallet:               serverWallet, // You'll need to create this
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create auth middleware: %w", err)
	}

	return &Server{
		Config: cfg,

		logger:        logger,
		storage:       activeStorage,
		monitor:       daemon,
		storageServer: storage.NewServer(logger, activeStorage, authMiddleware, storage.ServerOptions{Port: cfg.HTTPConfig.Port}),
	}, nil
}

// ListenAndServe starts the JSON-RPC server
func (s *Server) ListenAndServe() error {
	err := s.storageServer.Start()
	if err != nil {
		return fmt.Errorf("failed to start storage server: %w", err)
	}
	return nil
}

func makeLogger(cfg *Config, options *Options) *slog.Logger {
	if options.Logger != nil {
		return options.Logger
	}

	if !cfg.Logging.Enabled {
		return logging.New().Nop().Logger()
	}

	return logging.New().
		WithLevel(cfg.Logging.Level).
		WithHandler(cfg.Logging.Handler, os.Stdout).
		Logger()
}
