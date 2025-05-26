package storage

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/to"
)

var _ wdk.WalletStorage = (*WalletStorageManager)(nil)

type managedStorage struct {
	wdk.WalletStorageProvider
	settings *wdk.TableSettings
	user     *wdk.TableUser
}

func newMangedStorage(storage wdk.WalletStorageProvider) *managedStorage {
	return &managedStorage{
		WalletStorageProvider: storage,
	}
}

func (s *managedStorage) isAvailable() bool {
	return s.settings != nil && s.user != nil
}

func (s *managedStorage) makeAvailable(ctx context.Context, identityKey string) (*wdk.TableSettings, error) {
	if s.isAvailable() {
		return s.settings, nil
	}
	settings, err := s.MakeAvailable(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to make available storage: %w", err)
	}

	userResponse, err := s.FindOrInsertUser(ctx, identityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to find or insert user to storage %s: %w", settings.StorageName, err)
	}
	if userResponse.User.IdentityKey != identityKey {
		return nil, fmt.Errorf("storage %s returned user with different identity key (%s)", settings.StorageName, userResponse.User.IdentityKey)
	}

	s.settings = settings
	s.user = &userResponse.User

	return settings, nil
}

// WalletStorageManager provides methods for managing wallet active storage and backups.
// Also delivers authentication checking storage access to the wallet.
type WalletStorageManager struct {
	isAvailable   bool
	identityKey   string
	activeStorage *managedStorage
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
		activeStorage: newMangedStorage(active),
		identityKey:   identityKey,
	}
}

// MakeAvailable makes the storage available for the user.
func (m *WalletStorageManager) MakeAvailable(ctx context.Context) (*wdk.TableSettings, error) {
	if m.isAvailable {
		return m.activeStorage.settings, nil
	}
	settings, err := m.activeStorage.makeAvailable(ctx, m.identityKey)
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
		UserID:      to.Ptr(m.activeStorage.user.UserID),
		IdentityKey: m.identityKey,
		IsActive:    to.Ptr(m.activeStorage.settings.StorageIdentityKey == m.activeStorage.user.ActiveStorage),
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
