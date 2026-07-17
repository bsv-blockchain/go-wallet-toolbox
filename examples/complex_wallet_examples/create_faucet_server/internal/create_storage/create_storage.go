package create_storage

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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

// getExamplesDir returns the absolute path to the create_faucet_server directory
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

// CreateLocalStorage initializes a local GORM storage using the provided server private key
// and returns the storage provider and a cleanup function.
func CreateLocalStorage(ctx context.Context, network defs.BSVNetwork, serverPrivateKey string) (*storage.Provider, func(), error) {
	logger := slog.Default()

	cfg := infra.Defaults()
	cfg.ServerPrivateKey = serverPrivateKey
	if network == defs.NetworkTestnet {
		cfg.BSVNetwork = network
		cfg.Services = defs.DefaultServicesConfig(network)
	}

	// The database lives in a data/ subdirectory so deployments can persist it by
	// mounting that directory. SQLite runs in WAL mode by default, which keeps
	// -wal/-shm sibling files next to the database; a single-file mount would lose
	// committed-but-not-checkpointed transactions on restart.
	dataDir := filepath.Join(getExamplesDir(), "data")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("failed to create data directory %q: %w", dataDir, err)
	}
	cfg.DBConfig.SQLite.ConnectionString = filepath.Join(dataDir, SQLiteStorageFile)

	storageIdentityKey, err := wdk.IdentityKey(cfg.ServerPrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}

	activeServices := services.New(logger, cfg.Services)

	providerOptions := append(
		infra.GORMProviderOptionsFromConfig(&cfg),
		storage.WithLogger(logger),
		storage.WithBackgroundBroadcasterContext(ctx),
	)

	activeStorage, err := storage.NewGORMProvider(cfg.BSVNetwork, activeServices, providerOptions...)
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
				slog.ErrorContext(ctx, "failed to stop storage monitor", "error", err)
			}
		}
		activeStorage.Stop()
	}

	return activeStorage, cleanup, nil
}
