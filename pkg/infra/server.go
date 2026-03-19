package infra

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/go-softwarelab/common/pkg/must"

	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

// Server is a struct that holds the "infra" server configuration
type Server struct {
	Config Config

	logger        *slog.Logger
	services      *services.WalletServices
	storage       *storage.Provider
	storageServer *storage.Server
	monitor       *monitor.Daemon

	txBroadcastedCh <-chan wdk.CurrentTxStatus
	txProvenCh      <-chan wdk.CurrentTxStatus

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
		var tracingCleanup func()
		tracingCleanup, err = tracing.Enable(logger, "server", cfg.TracingConfig.DialAddr, cfg.TracingConfig.Sample)
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
		txBroadcastedCh chan wdk.CurrentTxStatus
		txProvenCh      chan wdk.CurrentTxStatus
	)
	if cfg.Monitor.Enabled {
		var monitorOpts []monitor.DaemonEventOption

		if cfg.Monitor.Events.TxBroadcasted.Enabled {
			txBroadcastedCh = make(chan wdk.CurrentTxStatus, cfg.Monitor.Events.TxBroadcasted.ChannelSize)
			monitorOpts = append(monitorOpts, monitor.WithBroadcastedTxChannel(txBroadcastedCh))

			cleanupFuncs = append(cleanupFuncs, func() {
				close(txBroadcastedCh)
			})
		}

		if cfg.Monitor.Events.TxProven.Enabled {
			txProvenCh = make(chan wdk.CurrentTxStatus, cfg.Monitor.Events.TxProven.ChannelSize)
			monitorOpts = append(monitorOpts, monitor.WithProvenTxChannel(txProvenCh))

			cleanupFuncs = append(cleanupFuncs, func() {
				close(txProvenCh)
			})
		}

		if cfg.Services.ChaintracksClient.Enabled {
			reorgChan := make(chan *chaintracks.ReorgEvent, 10)
			unsubReorg := activeServices.SubscribeReorgs(reorgChan)
			if unsubReorg != nil {
				monitorOpts = append(monitorOpts, monitor.WithReorgChannel(reorgChan))
				cleanupFuncs = append(cleanupFuncs, func() {
					unsubReorg()
					close(reorgChan)
				})
			} else {
				close(reorgChan)
			}

			tipChan := make(chan *chaintracks.BlockHeader, 10)
			unsubTips := activeServices.SubscribeTips(tipChan)
			if unsubTips != nil {
				monitorOpts = append(monitorOpts, monitor.WithTipChannel(tipChan))
				cleanupFuncs = append(cleanupFuncs, func() {
					unsubTips()
					close(tipChan)
				})
			} else {
				close(tipChan)
			}
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
		services:        activeServices,
		storage:         activeStorage,
		monitor:         daemon,
		storageServer:   storage.NewServer(logger, activeStorage, serverWallet, serverOptions),
		txBroadcastedCh: txBroadcastedCh,
		txProvenCh:      txProvenCh,
		cleanupFunc:     cleanupFuncs,
	}, nil
}

// ListenAndServe starts the JSON-RPC server
func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.txBroadcastedCh != nil {
		go s.consumeTxBroadcasted()
	}

	if s.txProvenCh != nil {
		go s.consumeTxProven()
	}

	if s.Config.Services.ChaintracksClient.Enabled {
		if err := s.services.StartChaintracks(context.Background()); err != nil {
			return fmt.Errorf("failed to start chaintracks: %w", err)
		}
	}

	if err := s.monitor.Start(ctx, s.Config.Monitor.Tasks.EnabledTasks()); err != nil {
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
			slog.String("reference", msg.Reference),
			slog.String("status", msg.Status.String()),
		)

		if msg.Error != nil {
			s.logger.Error(
				"tx broadcast error",
				slog.String("tx_id", msg.TxID),
				slog.Any("error", msg.Error.Errors),
			)
		}
	}
}

func (s *Server) consumeTxProven() {
	for msg := range s.txProvenCh {
		s.logger.Info(
			"tx proven",
			slog.String("tx_id", msg.TxID),
			slog.String("status", msg.Status.String()),
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
