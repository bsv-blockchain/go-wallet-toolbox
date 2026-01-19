package infra

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
)

// Server is a struct that holds the "infra" server configuration
type Server struct {
	Config Config

	logger        *slog.Logger
	storage       *storage.Provider
	storageServer *storage.Server
	monitor       *monitor.Daemon

	txBroadcastedCh <-chan defs.MonitorTaskResponse
	txProvenCh      <-chan defs.MonitorTaskResponse

	cleanupFunc []func()
}

// NewServer creates a new server instance with given options, like config file path or a prefix for environment variables
func NewServer(ctx context.Context, opts ...InitOption) (*Server, error) {
	options := defaultOptions()
	for _, option := range opts {
		option(&options)
	}

	cleanupFuncs := make([]func(), 0)

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

	if cfg.TracingConfig.Enabled {
		tracingCleanup, err := tracing.Enable(logger, "server", cfg.TracingConfig.DialAddr, cfg.TracingConfig.Sample)
		if err != nil {
			return nil, fmt.Errorf("failed to enable tracing: %w", err)
		}

		cleanupFuncs = append(cleanupFuncs, tracingCleanup)
	}

	activeServices := services.New(logger, cfg.Services)

	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}

	providerOptions := append(
		GORMProviderOptionsFromConfig(&cfg),
		storage.WithLogger(logger),
		storage.WithBackgroundBroadcasterContext(ctx),
	)

	activeStorage, err := storage.NewGORMProvider(cfg.BSVNetwork, activeServices, providerOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage provider: %w", err)
	}

	_, err = activeStorage.Migrate(ctx, cfg.Name, storageIdentityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate storage: %w", err)
	}

	serverWallet, err := wallet.New(cfg.BSVNetwork, cfg.ServerPrivateKey, activeStorage, wallet.WithLogger(logger), wallet.WithServices(activeServices))
	if err != nil {
		return nil, fmt.Errorf("failed to create server wallet: %w", err)
	}

	var (
		daemon          *monitor.Daemon
		txBroadcastedCh chan defs.MonitorTaskResponse
		txProvenCh      chan defs.MonitorTaskResponse
	)
	if cfg.Monitor.Enabled {
		var monitorOpts []monitor.DaemonCommunicationOption

		if cfg.Monitor.Events.TxBroadcasted.Enabled {
			txBroadcastedCh = make(chan defs.MonitorTaskResponse, cfg.Monitor.Events.TxBroadcasted.ChannelSize)
			monitorOpts = append(monitorOpts, monitor.WithBroadcastedTxChannel(txBroadcastedCh))

			cleanupFuncs = append(cleanupFuncs, func() {
				close(txBroadcastedCh)
			})
		}

		if cfg.Monitor.Events.TxProven.Enabled {
			txProvenCh = make(chan defs.MonitorTaskResponse, cfg.Monitor.Events.TxProven.ChannelSize)
			monitorOpts = append(monitorOpts, monitor.WithProvenTxChannel(txProvenCh))

			cleanupFuncs = append(cleanupFuncs, func() {
				close(txProvenCh)
			})
		}

		daemon, err = monitor.NewDaemonWithGORMLocker(ctx, logger, activeStorage, activeStorage.Database.DB, monitorOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create daemon: %w", err)
		}
	}

	// price is validated in config.Validate(), therefore we use must here.
	requestPrice := must.ConvertToIntFromUnsigned(cfg.HTTPConfig.RequestPrice)

	serverOptions := storage.ServerOptions{
		Port:     cfg.HTTPConfig.Port,
		Monetize: requestPrice != 0,
		CalculateRequestPrice: func(_ *http.Request) (int, error) {
			return requestPrice, nil
		},
	}

	return &Server{
		Config: cfg,

		logger:          logger,
		storage:         activeStorage,
		monitor:         daemon,
		storageServer:   storage.NewServer(logger, activeStorage, serverWallet, serverOptions),
		txBroadcastedCh: txBroadcastedCh,
		txProvenCh:      txProvenCh,
		cleanupFunc:     cleanupFuncs,
	}, nil
}

// ListenAndServe starts the JSON-RPC server
func (s *Server) ListenAndServe() error {
	if s.txBroadcastedCh != nil {
		go s.consumeTxBroadcasted()
	}

	if s.txProvenCh != nil {
		go s.consumeTxProven()
	}

	if err := s.monitor.Start(s.Config.Monitor.Tasks.EnabledTasks()); err != nil {
		return fmt.Errorf("failed to start storage monitor: %w", err)
	}

	err := s.storageServer.Start()
	if err != nil {
		return fmt.Errorf("failed to start storage server: %w", err)
	}

	return nil
}

// Cleanup releases all resources held by the server
func (s *Server) Cleanup() {
	s.logger.Info("Cleaning up resources...")

	if s.monitor != nil {
		_ = s.monitor.Stop()
	}

	for _, fn := range s.cleanupFunc {
		fn()
	}
}

func (s *Server) consumeTxBroadcasted() {
	for msg := range s.txBroadcastedCh {
		s.logger.Info(
			"tx broadcasted",
			slog.String("tx_id", msg.TxID),
			slog.String("status", msg.Status),
		)
	}
}

func (s *Server) consumeTxProven() {
	for msg := range s.txProvenCh {
		s.logger.Info(
			"tx proven",
			slog.String("tx_id", msg.TxID),
			slog.String("status", msg.Status),
		)
	}
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
