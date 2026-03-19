package internal

import (
	"context"
	"fmt"
	"log/slog"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"

	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

type Manager struct {
	ctx    context.Context
	config *fixtures.Config

	storageInfra  *StorageInfra
	remoteStorage wdk.WalletStorageProvider
	remoteCleanup func()
}

func NewManager(ctx context.Context, config *fixtures.Config) *Manager {
	return &Manager{
		ctx:    ctx,
		config: config,
	}
}

func (m *Manager) Ctx() context.Context {
	return m.ctx
}

func (m *Manager) SelectNetwork(network defs.BSVNetwork) {
	m.config.BSVNetwork = network
}

func (m *Manager) SelectStorageType(storageType fixtures.StorageType) error {
	switch storageType { //nolint:exhaustive // StorageTypeRemotePostgres is not yet implemented
	case fixtures.StorageTypeLocalSQLite:
		storage, err := CreateLocalStorage(m.ctx, m.config.BSVNetwork, m.config.ServerPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to create local storage: %w", err)
		}

		m.storageInfra = storage
	case fixtures.StorageTypeRemoteSQLite:
		return nil
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}

	return nil
}

func (m *Manager) WalletForUser(user fixtures.UserConfig) (sdk.Interface, error) {
	var storageProvider wdk.WalletStorageProvider

	switch {
	case m.remoteStorage != nil:
		storageProvider = m.remoteStorage
	case m.storageInfra != nil:
		storageProvider = m.storageInfra.Provider
	default:
		return nil, fmt.Errorf("no storage provider configured")
	}

	userWallet, err := wallet.New(m.config.BSVNetwork, user.PrivateKey(), storageProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet for user %s: %w", user.Name, err)
	}

	return userWallet, nil
}

func (m *Manager) Panic(err error, msg string) {
	slog.Default().Error(msg, "error", err.Error())
}

func (m *Manager) GetWalletConfigs() []fixtures.UserConfig {
	return []fixtures.UserConfig{
		m.config.Alice,
		m.config.Bob,
	}
}

func (m *Manager) GetBSVNetwork() defs.BSVNetwork {
	return m.config.BSVNetwork
}

func (m *Manager) Cleanup() {
	if m.remoteCleanup != nil {
		m.remoteCleanup()
	}
}

func (m *Manager) getServices() (*services.WalletServices, error) {
	if m.storageInfra != nil {
		return m.storageInfra.Services, nil
	}
	if m.remoteStorage != nil {
		serviceCfg := defs.DefaultServicesConfig(m.config.BSVNetwork)
		return services.New(slog.Default(), serviceCfg), nil
	}
	return nil, fmt.Errorf("no storage provider configured")
}
