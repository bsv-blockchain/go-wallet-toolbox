package internal

import (
	"context"
	"fmt"
	"log/slog"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox-manual-tests/internal/fixtures"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
)

type Manager struct {
	ctx    context.Context
	config *fixtures.Config

	storageInfra *StorageInfra
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
	return
}

func (m *Manager) SelectStorageType(storageType fixtures.StorageType) error {
	switch storageType {
	case fixtures.StorageTypeLocalSQLite:
		storage, err := CreateLocalStorage(m.ctx, m.config.BSVNetwork, m.config.ServerPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to create local storage: %w", err)
		}

		m.storageInfra = storage
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}

	return nil
}

func (m *Manager) WalletForUser(user fixtures.UserConfig) (sdk.Interface, error) {
	userWallet, err := wallet.New(m.config.BSVNetwork, user.PrivKey, m.storageInfra.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet for user %s: %w", user.Name, err)
	}

	return userWallet, nil
}

func (m *Manager) Panic(err error, msg string) {
	slog.Default().Error(msg, err.Error())
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
