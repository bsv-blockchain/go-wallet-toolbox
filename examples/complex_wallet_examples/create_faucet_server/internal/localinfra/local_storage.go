package localinfra

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
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

// getExampleDir returns the absolute path to this example's root directory
func getExampleDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}
	// this file: examples/complex_wallet_examples/create_faucet_server/internal/localinfra/local_storage.go
	// go up two levels to reach the create_faucet_server directory
	return filepath.Dir(filepath.Dir(filename))
}

// CreateLocalStorage initializes a local GORM storage with an ephemeral identity key
func CreateLocalStorage(ctx context.Context, network defs.BSVNetwork) (*StorageInfra, error) {
	logger := slog.Default()

	cfg := infra.Defaults()
	cfg.BSVNetwork = network
	if network == defs.NetworkTestnet {
		cfg.Services = defs.DefaultServicesConfig(network)
	}

	// Use a stable path for SQLite within the example root
	cfg.DBConfig.SQLite.ConnectionString = filepath.Join(getExampleDir(), "storage.sqlite")

	// Generate ephemeral server private key and identity
	privKey, err := ec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral private key: %w", err)
	}
	serverPrivHex := hex.EncodeToString(privKey.Serialize())
	storageIdentityKey, err := wdk.IdentityKey(serverPrivHex)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage identity key: %w", err)
	}

	activeServices := services.New(logger, cfg.Services)

	providerOptions := append(
		infra.GORMProviderOptionsFromConfig(&cfg),
		storage.WithLogger(logger),
		storage.WithBackgroundBroadcasterContext(ctx),
	)

	activeStorage, err := storage.NewGORMProvider(cfg.BSVNetwork, activeServices, providerOptions...)
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
