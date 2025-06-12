package sync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
)

type findOrInsertSyncState struct {
	logger *slog.Logger
	repo   Repository
	random wdk.Randomizer
}

func newFindOrInsertSyncState(logger *slog.Logger, repo Repository, random wdk.Randomizer) *findOrInsertSyncState {
	return &findOrInsertSyncState{
		logger: logging.Child(logger, "findOrInsertSyncState"),
		repo:   repo,
		random: random,
	}
}

func (f *findOrInsertSyncState) FindOrInsertSyncState(ctx context.Context, auth wdk.AuthID, storageIdentityKey, storageName string) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
	syncState, err := f.repo.FindSyncState(ctx, *auth.UserID, storageIdentityKey, storageName)
	if err != nil {
		return nil, fmt.Errorf("failed to find sync state: %w", err)
	}

	if syncState != nil {
		apiModel, err := syncState.ToWDK()
		if err != nil {
			return nil, fmt.Errorf("failed to convert sync state to WDK model: %w", err)
		}

		return &wdk.FindOrInsertSyncStateAuthResponse{
			SyncState: apiModel,
			IsNew:     false,
		}, nil
	}

	reference, err := f.random.Base64(12)
	if err != nil {
		return nil, fmt.Errorf("failed to generate reference number: %w", err)
	}

	syncState, err = f.repo.CreateSyncState(ctx, &entity.SyncState{
		UserID:             *auth.UserID,
		StorageIdentityKey: storageIdentityKey,
		StorageName:        storageName,
		Status:             wdk.SyncStatusUnknown,
		Reference:          reference,
		SyncMap:            wdk.NewSyncMap(),

		// TODO: Check when Satoshis field should be set
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sync state: %w", err)
	}

	apiModel, err := syncState.ToWDK()
	if err != nil {
		return nil, fmt.Errorf("failed to convert sync state to WDK model: %w", err)
	}

	return &wdk.FindOrInsertSyncStateAuthResponse{
		SyncState: apiModel,
		IsNew:     true,
	}, nil
}
