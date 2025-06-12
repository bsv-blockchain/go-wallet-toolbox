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
		// TODO: We need to revisit this panic, as in TS the active storage is optional an it's almost never assigned during construction.
		panic("activeStorage storage must be provided")
	}

	if is.BlankString(identityKey) {
		panic("identity key must be provided and cannot be empty")
	}

	return &WalletStorageManager{
		activeStorage: managed.NewManagedStorage(active),
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

// SyncToWriter synchronizes wallet data from the active storage to the provided writer storage provider.
// NOTE: reader(source) => writer(backup)
func (m *WalletStorageManager) SyncToWriter(ctx context.Context, auth wdk.AuthID, writer wdk.WalletStorageProvider) (inserts, updates int, err error) {
	// TODO: add locking mechanism to ensure that the active storage is not being modified while syncing

	writerSettings, err := writer.MakeAvailable(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to make writer storage available: %w", err)
	}

	reader := m.getActiveReader()
	readerSettings, err := reader.MakeAvailable(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to make reader storage available: %w", err)
	}

	if writerSettings.Chain != readerSettings.Chain {
		return 0, 0, fmt.Errorf("cannot sync between different chains: reader chain %s, writer chain %s", readerSettings.Chain, writerSettings.Chain)
	}

	// TODO: implement looping mechanism to handle multiple sync chunks
	for range 1 {
		writerSyncState, err := writer.FindOrInsertSyncStateAuth(ctx, auth, readerSettings.StorageIdentityKey, readerSettings.StorageName)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to find or insert sync state auth: %w", err)
		}

		syncState := writerSyncState.SyncState

		var offsets []wdk.SyncOffsets
		syncMap, err := wdk.NewSyncMapFromJSON([]byte(syncState.SyncMap))
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse sync map: %w", err)
		}
		for _, entityName := range wdk.AllEntityNames {
			syncMapEntity, ok := syncMap[entityName]
			if !ok {
				continue
			}

			offsets = append(offsets, wdk.SyncOffsets{
				Name:   entityName,
				Offset: syncMapEntity.Count,
			})
		}

		getSyncChunkArgs := wdk.RequestSyncChunkArgs{
			FromStorageIdentityKey: readerSettings.StorageIdentityKey,
			ToStorageIdentityKey:   writerSettings.StorageIdentityKey,
			IdentityKey:            auth.IdentityKey,

			Since:        syncState.When,
			MaxRoughSize: 10_000_000, // ~10 MB
			MaxItems:     1000,
			Offsets:      offsets,
		}

		chunk, err := reader.GetSyncChunk(ctx, getSyncChunkArgs)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get sync chunk from reader storage: %w", err)
		}

		processChunkResult, err := writer.ProcessSyncChunk(ctx, getSyncChunkArgs, chunk)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to process sync chunk in writer storage: %w", err)
		}

		if processChunkResult.Done || (processChunkResult.Updates == 0 && processChunkResult.Inserts == 0) {
			// No more updates to process, we can stop syncing
			break
		}

		inserts += processChunkResult.Inserts
		updates += processChunkResult.Updates

		// TODO Log the sync chunk received (it needs the Manager to include a logger)

	}
	return inserts, updates, nil
}

func (m *WalletStorageManager) getActiveReader() wdk.WalletStorageProvider {
	// TODO: add locking mechanism
	return m.activeStorage
}

func (m *WalletStorageManager) getActiveWriter() wdk.WalletStorageProvider {
	// TODO: add locking mechanism
	return m.activeStorage
}
