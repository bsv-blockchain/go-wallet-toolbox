package infra

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bsv-blockchain/go-chaintracks/chaintracks"
	"github.com/go-softwarelab/common/pkg/must"

	"github.com/bsv-blockchain/go-wallet-toolbox/internal/config"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
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

	// Metrics reuse the tracing OTLP endpoint; tracing.enabled gates spans only.
	if cfg.Observability.Metrics.Enabled {
		var metricsCleanup func()
		metricsCleanup, err = tracing.EnableMetrics(logger, "server", cfg.TracingConfig.DialAddr,
			time.Duration(must.ConvertToInt64FromUnsigned(cfg.Observability.Metrics.ExportIntervalSeconds))*time.Second)
		if err != nil {
			return nil, fmt.Errorf("failed to enable metrics: %w", err)
		}

		cleanupFuncs = append(cleanupFuncs, metricsCleanup)
	}

	// The throughput UTXO strategy on SQLite tops out far below its rated
	// loads (single-writer); Postgres is the intended engine.
	if cfg.UTXOManagement.Enabled() && cfg.DBConfig.Engine == defs.DBTypeSQLite {
		logger.Warn("utxo_management strategy=throughput with the SQLite engine: expect single-writer contention; use Postgres for rated loads")
	}

	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}

	if cfg.Services.Arcade.Enabled && cfg.Services.Arcade.CallbackToken == "" {
		cfg.Services.Arcade.CallbackToken = wdk.DeriveArcadeCallbackToken(storageIdentityKey)
	}

	activeServices := services.New(logger, cfg.Services)

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

		if cfg.Services.Arcade.Enabled {
			monitorOpts = append(monitorOpts, monitor.WithBroadcastEventStream(activeServices))
		}

		daemon, err = monitor.NewDaemonWithGORMLocker(ctx, logger, activeStorage, activeStorage.Database.DB, monitorOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create daemon: %w", err)
		}
	}

	// price is validated in config.Validate(), therefore we use must here.
	requestPrice := must.ConvertToIntFromUnsigned(cfg.HTTPConfig.RequestPrice)

	serverOptions := storage.ServerOptions{
		Port:                cfg.HTTPConfig.Port,
		MaxRequestBodyBytes: cfg.HTTPConfig.MaxRequestBodyBytes,
		CORS:                cfg.HTTPConfig.CORS,
		Monetize:            requestPrice != 0,
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
		go s.consumeTxBroadcasted(ctx)
	}

	if s.txProvenCh != nil {
		go s.consumeTxProven(ctx)
	}

	// StartBackgroundServices starts the Arcade circuit-breaker health probe (when
	// the broadcast router is enabled) and chaintracks (no-op when disabled). Prefer
	// this over StartChaintracks alone so half-open recovery does not rely solely on
	// opportunistic broadcast trials after the circuit opens.
	if err := s.services.StartBackgroundServices(ctx); err != nil {
		return fmt.Errorf("failed to start background services: %w", err)
	}

	if s.monitor != nil {
		if err := s.monitor.Start(ctx, s.Config.Monitor.Tasks.EnabledTasks()); err != nil {
			return fmt.Errorf("failed to start storage monitor: %w", err)
		}
	}

	err := s.storageServer.Start()
	if err != nil {
		return fmt.Errorf("failed to start storage server: %w", err)
	}

	return nil
}

// Cleanup releases all resources held by the server
func (s *Server) Cleanup() {
	s.logger.InfoContext(context.Background(), "Cleaning up resources...")

	if s.monitor != nil {
		_ = s.monitor.Stop()
	}

	for _, fn := range s.cleanupFunc {
		fn()
	}
}

func (s *Server) consumeTxBroadcasted(ctx context.Context) {
	for msg := range s.txBroadcastedCh {
		s.logger.InfoContext(
			ctx,
			"tx broadcasted",
			slog.String("tx_id", msg.TxID),
			slog.String("reference", msg.Reference),
			slog.String("status", msg.Status.String()),
		)

		if msg.Error != nil {
			s.logger.ErrorContext(
				ctx,
				"tx broadcast error",
				slog.String("tx_id", msg.TxID),
				slog.Any("error", msg.Error.Errors),
			)
		}
	}
}

func (s *Server) consumeTxProven(ctx context.Context) {
	for msg := range s.txProvenCh {
		s.logger.InfoContext(
			ctx,
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
