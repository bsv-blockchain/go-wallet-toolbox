package infra

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"go.opentelemetry.io/otel/trace"
)

// Server is a struct that holds the "infra" server configuration
type Server struct {
	Config Config

	ctx           context.Context
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

	ctx, span := options.Tracer.Start(ctx, "infra.NewServer")
	defer span.End()

	ctx, configSpan := options.Tracer.Start(ctx, "infra.loadConfig")
	loader := config.NewLoader(Defaults, options.EnvPrefix)
	if options.ConfigFile != "" {
		if err := loader.SetConfigFilePath(options.ConfigFile); err != nil {
			configSpan.End()
			return nil, fmt.Errorf("failed to set config file path: %w", err)
		}
	}
	cfg, err := loader.Load()
	if err != nil {
		configSpan.End()
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		configSpan.End()
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	configSpan.End()

	logger := logging.Child(makeLogger(&cfg, &options), "infra")

	ctx, servicesSpan := options.Tracer.Start(ctx, "infra.initServices")
	activeServices := services.New(logger, cfg.Services)
	servicesSpan.End()

	ctx, identitySpan := options.Tracer.Start(ctx, "infra.initIdentityKey")
	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		identitySpan.End()
		return nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}
	identitySpan.End()

	ctx, storageSpan := options.Tracer.Start(ctx, "infra.initStorage")
	providerOptions := append(
		GORMProviderOptionsFromConfig(&cfg),
		storage.WithLogger(logger),
		storage.WithBackgroundBroadcasterContext(ctx),
	)

	activeStorage, err := storage.NewGORMProvider(cfg.BSVNetwork, activeServices, providerOptions...)
	if err != nil {
		storageSpan.End()
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}
	if _, err := activeStorage.Migrate(ctx, cfg.Name, storageIdentityKey); err != nil {
		storageSpan.End()
		return nil, fmt.Errorf("failed to migrate storage: %w", err)
	}
	storageSpan.End()

	ctx, walletSpan := options.Tracer.Start(ctx, "infra.initWallet")
	serverWallet, err := wallet.New(cfg.BSVNetwork, cfg.ServerPrivateKey, activeStorage,
		wallet.WithLogger(logger),
		wallet.WithServices(activeServices),
	)
	if err != nil {
		walletSpan.End()
		return nil, fmt.Errorf("failed to create server wallet: %w", err)
	}
	walletSpan.End()

	var daemon *monitor.Daemon
	if cfg.Monitor.Enabled {
		ctx, monitorSpan := options.Tracer.Start(ctx, "infra.initMonitor")
		daemon, err = monitor.NewDaemonWithGORMLocker(ctx, logger, activeStorage, activeStorage.Database.DB)
		if err != nil {
			monitorSpan.End()
			return nil, fmt.Errorf("failed to create daemon: %w", err)
		}
		if err := daemon.Start(ctx, cfg.Monitor.Tasks.EnabledTasks()); err != nil {
			monitorSpan.End()
			return nil, fmt.Errorf("failed to start storage monitor: %w", err)
		}
		monitorSpan.End()
	}

	return &Server{
		Config:  cfg,
		ctx:     ctx,
		logger:  logger,
		storage: activeStorage,
		monitor: daemon,
		storageServer: storage.NewServer(logger, activeStorage, serverWallet, storage.ServerOptions{
			Port:   cfg.HTTPConfig.Port,
			Tracer: options.Tracer,
		}),
	}, nil
}

// ListenAndServe starts the JSON-RPC server
func (s *Server) ListenAndServe() error {
	ctx, span := s.ctx, trace.SpanFromContext(s.ctx)
	if !span.SpanContext().IsValid() {
		var newSpan trace.Span
		ctx, newSpan = s.ctx, trace.SpanFromContext(s.ctx)
		defer newSpan.End()
	}

	if err := s.storageServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start storage server: %w", err)
	}

	go func() {
		<-ctx.Done()
		if s.monitor != nil {
			_ = s.monitor.Stop()
		}
		if s.storageServer != nil {
			_ = s.storageServer.Stop(ctx)
		}
	}()

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
