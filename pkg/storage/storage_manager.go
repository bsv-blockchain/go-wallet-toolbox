package storage

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/managed"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/to"
)

var _ wdk.WalletStorage = (*WalletStorageManager)(nil)

// WalletStorageManager provides methods for managing wallet active storage and backups.
// Also delivers authentication checking storage access to the wallet.
type WalletStorageManager struct {
	isAvailable   bool
	identityKey   string
	activeStorage *managed.Storage
}

// NewWalletStorageManager initializes a WalletStorageManager with an identity key and an active storage provider.
// Active storage and identity key must be provided, and it will panic if they are not.
func NewWalletStorageManager(identityKey string, active wdk.WalletStorageProvider, backups ...wdk.WalletStorageProvider) *WalletStorageManager {
	if len(backups) > 0 {
		panic("handling backup storages is not implemented yet")
	}

	if active == nil {
		panic("activeStorage storage must be provided")
	}

	if is.BlankString(identityKey) {
		panic("identity key must be provided and cannot be empty")
	}

	return &WalletStorageManager{
		activeStorage: managed.NewMangedStorage(active),
		identityKey:   identityKey,
	}
}

// MakeAvailable makes the storage available for the user.
func (m *WalletStorageManager) MakeAvailable(ctx context.Context) (*wdk.TableSettings, error) {
	if m.isAvailable {
		return m.activeStorage.Settings, nil
	}
	settings, err := m.activeStorage.MakeAvailableStorage(ctx, m.identityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to make available active storage: %w", err)
	}

	// TODO: verify if active storage is configured as user active storage

	m.isAvailable = true
	return settings, nil
}

// GetAuth retrieves the authentication identity of the user after ensuring the storage is available and active.
func (m *WalletStorageManager) GetAuth(ctx context.Context) (wdk.AuthID, error) {
	_, err := m.MakeAvailable(ctx)
	if err != nil {
		return wdk.AuthID{}, fmt.Errorf("failed to make storage available: %w", err)
	}

	// TODO: handle that the active storage is not really an active storage

	return wdk.AuthID{
		UserID:      to.Ptr(m.activeStorage.User.UserID),
		IdentityKey: m.identityKey,
		IsActive:    to.Ptr(m.activeStorage.Settings.StorageIdentityKey == m.activeStorage.User.ActiveStorage),
	}, nil
}

func (m *WalletStorageManager) getActiveReader() wdk.WalletStorageProvider {
	// TODO: add locking mechanism
	return m.activeStorage
}

func (m *WalletStorageManager) getActiveWriter() wdk.WalletStorageProvider {
	// TODO: add locking mechanism
	return m.activeStorage
}
