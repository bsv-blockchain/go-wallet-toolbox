package example_setup

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
)

type StorageInfra struct {
	Provider *storage.Provider
	Monitor  *monitor.Daemon
	Services *services.WalletServices
	Logger   *slog.Logger
}

// Close gracefully shuts down the storage infrastructure
func (s *StorageInfra) Close(ctx context.Context) error {
	if s.Monitor != nil {
		s.Logger.Info("Stopping monitor daemon...")
		if err := s.Monitor.Stop(); err != nil {
			return fmt.Errorf("failed to stop monitor: %w", err)
		}
	}

	s.Logger.Info("Stopping storage provider...")
	s.Provider.Stop()

	// Close the underlying database connection
	if s.Provider.Database != nil && s.Provider.Database.DB != nil {
		sqlDB, err := s.Provider.Database.DB.DB()
		if err != nil {
			s.Logger.Warn("Failed to get underlying SQL database", "error", err)
		} else {
			if err := sqlDB.Close(); err != nil {
				s.Logger.Warn("Failed to close database connection", "error", err)
			}
		}
	}

	s.Logger.Info("StorageInfra successfully closed")
	return nil
}

func CreateLocalStorage(
	ctx context.Context,
	logger *slog.Logger,
	network defs.BSVNetwork,
	serverPrivateKey string,
	sqlitePath string,
) (*StorageInfra, error) {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("Initializing infra configuration...")
	cfg := infra.Defaults()
	cfg.ServerPrivateKey = serverPrivateKey
	cfg.DBConfig.SQLite.ConnectionString = sqlitePath
	if network == defs.NetworkTestnet {
		cfg.BSVNetwork = network
		cfg.Services = defs.DefaultServicesConfig(network)
	}

	logger.Info("Deriving identity key...")
	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}

	logger.Info("Setting up wallet services...")
	activeServices := services.New(logger, cfg.Services)

	options := append(
		infra.GORMProviderOptionsFromConfig(&cfg),
		storage.WithLogger(logger),
		storage.WithBackgroundBroadcasterContext(ctx),
	)

	logger.Info("Creating GORM storage provider...")
	activeStorage, err := storage.NewGORMProvider(cfg.BSVNetwork, activeServices, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	logger.Info("Running storage migration...")
	if _, err = activeStorage.Migrate(ctx, cfg.Name, storageIdentityKey); err != nil {
		return nil, fmt.Errorf("failed to migrate storage: %w", err)
	}

	var daemon *monitor.Daemon
	if cfg.Monitor.Enabled {
		logger.Info("Initializing monitor daemon...")
		daemon, err = monitor.NewDaemonWithGORMLocker(ctx, logger, activeStorage, activeStorage.Database.DB)
		if err != nil {
			return nil, fmt.Errorf("failed to create daemon: %w", err)
		}

		logger.Info("Starting monitor tasks...")
		if err = daemon.Start(cfg.Monitor.Tasks.EnabledTasks()); err != nil {
			return nil, fmt.Errorf("failed to start storage monitor: %w", err)
		}
	}

	logger.Info("StorageInfra successfully initialized.")
	return &StorageInfra{
		Provider: activeStorage,
		Monitor:  daemon,
		Services: activeServices,
		Logger:   logger,
	}, nil
}
