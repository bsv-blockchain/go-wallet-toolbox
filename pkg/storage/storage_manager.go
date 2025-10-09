package storage

import (
	"context"
	"fmt"
	"log/slog"
	stdslices "slices"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/managed"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/sync"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

var _ wdk.WalletStorage = (*WalletStorageManager)(nil)

// WalletStorageManager provides methods for managing wallet active storage and backups.
// Also delivers authentication checking storage access to the wallet.
type WalletStorageManager struct {
	isAvailable        bool
	identityKey        string
	activeStorage      *managed.Storage
	logger             *slog.Logger
	stores             []*managed.Storage
	backups            []*managed.Storage
	conflictingActives []*managed.Storage
}

// NewWalletStorageManager initializes a WalletStorageManager with an identity key and an active storage provider.
// Active storage and identity key must be provided, and it will panic if they are not.
func NewWalletStorageManager(identityKey string, logger *slog.Logger, active wdk.WalletStorageProvider, backups ...wdk.WalletStorageProvider) *WalletStorageManager {
	if is.BlankString(identityKey) {
		panic("identity key must be provided and cannot be empty")
	}

	var stores []*managed.Storage
	storesNum := len(backups) + to.IfThen(active != nil, 1).ElseThen(0)
	if storesNum > 0 {
		stores = make([]*managed.Storage, 0, storesNum)
		if active != nil {
			stores = append(stores, managed.NewManagedStorage(active))
		}
		for _, b := range backups {
			stores = append(stores, managed.NewManagedStorage(b))
		}
	}

	logger = logging.Child(logger, "StorageManager")

	return &WalletStorageManager{
		identityKey: identityKey,
		logger:      logger,

		stores: stores,
	}
}

// IsActiveEnabled The active storage is "enabled" only if its `storageIdentityKey` matches the user's currently selected `activeStorage`,
// and only if there are no stores with conflicting `activeStorage` selections.
//
// A wallet may be created without including the user's currently selected active storage. This allows readonly access to their wallet data.
//
// In addition, if there are conflicting `activeStorage` selections among backup storage providers then the active remains disabled.
func (m *WalletStorageManager) IsActiveEnabled() bool {
	return m.activeStorage != nil &&
		m.activeStorage.Settings.StorageIdentityKey == m.activeStorage.User.ActiveStorage &&
		len(m.conflictingActives) == 0
}

// MakeAvailable makes the storage available for the user.
func (m *WalletStorageManager) MakeAvailable(ctx context.Context) (*wdk.TableSettings, error) {
	if m.isAvailable {
		return m.activeStorage.Settings, nil
	}

	if len(m.stores) == 0 {
		return nil, fmt.Errorf("no storage providers configured")
	}

	m.activeStorage = m.stores[0] // first storage is the active storage candidate
	_, err := m.activeStorage.MakeAvailableStorage(ctx, m.identityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to make available active storage: %w", err)
	}

	m.backups = nil
	m.conflictingActives = nil

	tmpBackups := make([]*managed.Storage, 0, len(m.stores)-1)
	for _, store := range m.stores[1:] {
		_, err := store.MakeAvailableStorage(ctx, m.identityKey)
		if err != nil {
			return nil, fmt.Errorf("failed to make available storage: %w", err)
		}

		if store.ThinksItIsActive() {
			// swapping active storage
			tmpBackups = append(tmpBackups, m.activeStorage)
			m.activeStorage = store
		} else {
			tmpBackups = append(tmpBackups, store)
		}
	}

	for _, backup := range tmpBackups {
		if !backup.ThinksActiveStorageIs(m.activeStorage.Settings.StorageIdentityKey) {
			m.conflictingActives = append(m.conflictingActives, backup)
		} else {
			m.backups = append(m.backups, backup)
		}
	}

	m.isAvailable = true

	return m.activeStorage.Settings, nil
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

// SyncToWriter synchronizes wallet data from the active storage to the provided writer storage provider.
// NOTE: reader(source) => writer(backup)
func (m *WalletStorageManager) SyncToWriter(ctx context.Context, writer wdk.WalletStorageProvider, opts ...wdk.SyncToWriterOption) (inserts, updates int, err error) {
	// TODO: add locking mechanism to ensure that the active storage is not being modified while syncing

	options := to.OptionsWithDefault(wdk.SyncToWriterOptions{
		MaxSyncChunkSize: wdk.MaxSyncChunkSize,
		MaxSyncItems:     wdk.MaxSyncItems,
		ReaderFactory: func() wdk.WalletStorageProvider {
			return m.getActiveReader()
		},
	}, opts...)

	if writer == nil {
		return 0, 0, fmt.Errorf("writer wallet storage must be provided, it's nil")
	}

	m.logger.Info("starting sync from active storage to writer storage", slog.String("identityKey", m.identityKey))

	reader := options.ReaderFactory()
	if reader == nil {
		return 0, 0, fmt.Errorf("no active storage available to read from")
	}
	auth := wdk.AuthID{IdentityKey: m.identityKey}

	inserts, updates, err = sync.NewReaderToWriter(m.logger).Sync(ctx, auth, reader, writer, options.MaxSyncChunkSize, options.MaxSyncItems)
	if err != nil {
		err = fmt.Errorf("failed to sync from reader to writer: %w", err)
	}

	m.logger.Info("completed sync from active storage to writer storage",
		slog.Int("inserts", inserts),
		slog.Int("updates", updates),
		slog.String("identityKey", m.identityKey),
	)

	return
}

// SetActive Updates backups and switches to new active storage provider from among current backup providers.
// Also resolves conflicting actives
func (m *WalletStorageManager) SetActive(ctx context.Context, storageIdentityKey string) error {
	if is.BlankString(storageIdentityKey) {
		return fmt.Errorf("storage identity key must be provided and cannot be empty")
	}

	if m.activeStorage != nil && m.activeStorage.Settings.StorageIdentityKey == storageIdentityKey {
		//already active
		return nil
	}

	if _, err := m.MakeAvailable(ctx); err != nil {
		return fmt.Errorf("failed to make storage available: %w", err)
	}

	newActiveIndex := stdslices.IndexFunc(m.stores, func(storage *managed.Storage) bool {
		return storage.Settings.StorageIdentityKey == storageIdentityKey
	})
	if newActiveIndex == -1 {
		return fmt.Errorf("storage with identity key %s not found among managed storages", storageIdentityKey)
	}

	newActive := m.stores[newActiveIndex]
	// TODO: add locking mechanism

	if len(m.conflictingActives) > 0 {
		// Merge state from conflicting actives into `newActive`.

		// Handle case where new active is current active to resolve conflicts.
		// And where new active is one of the current conflict actives.
		m.conflictingActives = append(m.conflictingActives, m.activeStorage)
		// Remove the new active from conflicting actives and
		// set new active as the conflicting active that matches the target `storageIdentityKey`
		m.conflictingActives = slices.Filter(m.conflictingActives, func(item *managed.Storage) bool {
			return item.Settings.StorageIdentityKey != storageIdentityKey
		})

		// Merge state from conflicting actives into `newActive`.
		for _, conflict := range m.conflictingActives {
			m.logger.Info("merging state from conflicting actives",
				slog.String("from", conflict.Settings.StorageIdentityKey),
				slog.String("to", newActive.Settings.StorageIdentityKey),
			)

			if _, _, err := m.SyncToWriter(ctx, newActive, wdk.WithSyncReader(conflict)); err != nil {
				return fmt.Errorf("failed to sync from conflicting active %q to new active %q: %w",
					conflict.Settings.StorageIdentityKey, newActive.Settings.StorageIdentityKey, err)
			}
		}

		m.logger.Info("propagate merged active state to non-actives")
	} else {
		m.logger.Info("backup current active state then set new active")
	}

	// If there were conflicting actives,
	// Push state merged from all merged actives into newActive to all stores other than the now single active.
	// Otherwise,
	// Push state from current active to all other stores.
	backupSource := to.IfThen(len(m.conflictingActives) > 0, newActive).ElseThen(m.activeStorage)
	backupIdentityKey := backupSource.Settings.StorageIdentityKey

	err := backupSource.SetActive(ctx, wdk.AuthID{IdentityKey: m.identityKey, UserID: to.Ptr(backupSource.User.UserID)}, storageIdentityKey)
	if err != nil {
		return fmt.Errorf("failed to set active storage in backup source %q: %w", backupIdentityKey, err)
	}

	for _, store := range m.stores {
		// Update cached user.activeStorage of all stores
		store.User.ActiveStorage = storageIdentityKey

		if !store.HasStorageIdentityKey(backupIdentityKey) {
			// If this store is not the backupSource store push state from backupSource to this store.
			if _, _, err := m.SyncToWriter(ctx, store, wdk.WithSyncReader(backupSource)); err != nil {
				return fmt.Errorf("failed to sync from backup source %q to store %q: %w",
					backupIdentityKey, store.Settings.StorageIdentityKey, err)
			}
		}
	}

	m.isAvailable = false
	if _, err := m.MakeAvailable(ctx); err != nil {
		return fmt.Errorf("failed to make storage available after setting new active: %w", err)
	}

	return nil
}

// GetActive returns the currently active storage provider, or nil if none is set.
func (m *WalletStorageManager) GetActive() wdk.WalletStorageProvider {
	if m.activeStorage == nil {
		return nil
	}

	return m.activeStorage.WalletStorageProvider
}

// GetActiveStore returns the identity key of the currently active storage provider, or an empty string if none is set.
func (m *WalletStorageManager) GetActiveStore() string {
	if m.activeStorage == nil {
		return ""
	}

	return m.activeStorage.Settings.StorageIdentityKey
}

func (m *WalletStorageManager) getActiveReader() wdk.WalletStorageProvider {
	if m.activeStorage == nil {
		return nil
	}

	// TODO: add locking mechanism
	return m.activeStorage
}

func (m *WalletStorageManager) getActiveWriter() wdk.WalletStorageProvider {
	if m.activeStorage == nil {
		return nil
	}

	// TODO: add locking mechanism
	return m.activeStorage
}

// FindOutputBaskets finds output baskets for the authenticated user based on the provided filters.
// This is an alias to FindOutputBasketsAuth for TS-version compatibility.
func (m *WalletStorageManager) FindOutputBaskets(ctx context.Context, filters wdk.FindOutputBasketsArgs) (wdk.TableOutputBaskets, error) {
	return m.FindOutputBasketsAuth(ctx, filters)
}

// FindOutputs finds outputs for the authenticated user based on the provided filters.
// This is an alias to FindOutputsAuth for TS-version compatibility.
func (m *WalletStorageManager) FindOutputs(ctx context.Context, filters wdk.FindOutputsArgs) (wdk.TableOutputs, error) {
	return m.FindOutputsAuth(ctx, filters)
}
