package internal

import (
	"context"
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
)

type StorageInfra struct {
	Provider *storage.Provider
	Monitor  *monitor.Daemon
	Services *services.WalletServices
}

func CreateLocalStorage(ctx context.Context, network defs.BSVNetwork, serverPrivateKey string) (*StorageInfra, error) {
	logger := slog.Default()

	cfg := infra.Defaults()
	cfg.ServerPrivateKey = serverPrivateKey
	if network == defs.NetworkTestnet {
		cfg.BSVNetwork = network
		cfg.Services = defs.DefaultServicesConfig(network)
	}

	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}

	activeServices := services.New(logger, cfg.Services)

	activeStorage, err := storage.NewGORMProvider(ctx, logger, storage.GORMProviderConfig{
		DB:                    cfg.DBConfig,
		Chain:                 cfg.BSVNetwork,
		FeeModel:              cfg.FeeModel,
		Commission:            cfg.Commission,
		Services:              activeServices,
		SynchronizeTxStatuses: cfg.SynchronizeTxStatuses,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
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

	return &StorageInfra{
		Provider: activeStorage,
		Monitor:  daemon,
		Services: activeServices,
	}, nil
}
