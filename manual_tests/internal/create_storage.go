package internal

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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
	cfg.BSVNetwork = network
	cfg.Services = defs.DefaultServicesConfig(network)

	networkSuffix := strings.ToLower(string(network))
	cfg.DBConfig.SQLite.ConnectionString = fmt.Sprintf("./storage_%s.sqlite", networkSuffix)

	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}

	activeServices := services.New(logger, cfg.Services)

	options := append(
		infra.GORMProviderOptionsFromConfig(&cfg),
		storage.WithLogger(logger),
		storage.WithBackgroundBroadcasterContext(ctx),
	)

	activeStorage, err := storage.NewGORMProvider(cfg.BSVNetwork, activeServices, options...)
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

		if err = daemon.Start(ctx, cfg.Monitor.Tasks.EnabledTasks()); err != nil {
			return nil, fmt.Errorf("failed to start storage monitor: %w", err)
		}
	}

	return &StorageInfra{
		Provider: activeStorage,
		Monitor:  daemon,
		Services: activeServices,
	}, nil
}
