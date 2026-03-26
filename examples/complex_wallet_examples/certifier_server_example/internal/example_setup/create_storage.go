package example_setup

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/monitor"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	SQLiteStorageFile = "storage.sqlite"
)

func getExamplesDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}

	const parentLevels = 3
	for range parentLevels {
		filename = filepath.Dir(filename)
	}
	return filename
}

func CreateLocalStorage(ctx context.Context, network defs.BSVNetwork, serverPrivateKey string) (*storage.Provider, func(), error) {
	logger := slog.Default()

	cfg := infra.Defaults()
	cfg.ServerPrivateKey = serverPrivateKey
	if network == defs.NetworkTestnet {
		cfg.BSVNetwork = network
		cfg.Services = defs.DefaultServicesConfig(network)
	}

	cfg.DBConfig.SQLite.ConnectionString = filepath.Join(getExamplesDir(), SQLiteStorageFile)

	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}

	activeServices := services.New(logger, cfg.Services)

	options := append(
		infra.GORMProviderOptionsFromConfig(&cfg),
		storage.WithLogger(logger),
		storage.WithBackgroundBroadcasterContext(ctx),
	)

	activeStorage, err := storage.NewGORMProvider(cfg.BSVNetwork, activeServices, options...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create storage: %w", err)
	}

	_, err = activeStorage.Migrate(ctx, cfg.Name, storageIdentityKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to migrate storage: %w", err)
	}

	var daemon *monitor.Daemon
	if cfg.Monitor.Enabled {
		daemon, err = monitor.NewDaemonWithGORMLocker(ctx, logger, activeStorage, activeStorage.Database.DB)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create daemon: %w", err)
		}

		if err = daemon.Start(ctx, cfg.Monitor.Tasks.EnabledTasks()); err != nil {
			return nil, nil, fmt.Errorf("failed to start storage monitor: %w", err)
		}
	}

	cleanup := func() {
		if daemon != nil {
			if err := daemon.Stop(); err != nil {
				slog.Error(fmt.Sprintf("failed to stop storage monitor: %v", err))
			}
		}
		activeStorage.Stop()
	}

	return activeStorage, cleanup, nil
}
