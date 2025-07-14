package sync

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
)

const (
	referenceLength = 12
)

type FindOrInsertSyncState struct {
	repo               Repository
	random             wdk.Randomizer
	userID             int
	storageIdentityKey string
	storageName        string
}

func NewFindOrInsertSyncState(repo Repository, random wdk.Randomizer, userID int, storageIdentityKey, storageName string) *FindOrInsertSyncState {
	return &FindOrInsertSyncState{
		repo:               repo,
		random:             random,
		userID:             userID,
		storageIdentityKey: storageIdentityKey,
		storageName:        storageName,
	}
}

func (f *FindOrInsertSyncState) FindOrInsertSyncState(ctx context.Context) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
	syncState, err := f.repo.FindSyncState(ctx, f.userID, f.storageIdentityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to find sync state: %w", err)
	}

	if syncState != nil {
		return f.stateToResult(syncState, false)
	}

	syncState, err = f.createNewState(ctx)
	if err != nil {
		return nil, err
	}

	return f.stateToResult(syncState, true)
}

func (f *FindOrInsertSyncState) stateToResult(syncState *entity.SyncState, isNew bool) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
	apiModel, err := syncState.ToWDK()
	if err != nil {
		return nil, fmt.Errorf("failed to convert sync state to WDK model: %w", err)
	}

	return &wdk.FindOrInsertSyncStateAuthResponse{
		SyncState: apiModel,
		IsNew:     isNew,
	}, nil
}

func (f *FindOrInsertSyncState) createNewState(ctx context.Context) (*entity.SyncState, error) {
	reference, err := f.random.Base64(referenceLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate reference number: %w", err)
	}

	syncState, err := f.repo.CreateSyncState(ctx, &entity.SyncState{
		UserID:             f.userID,
		StorageIdentityKey: f.storageIdentityKey,
		StorageName:        f.storageName,
		Status:             wdk.SyncStatusUnknown,
		Reference:          reference,
		SyncMap:            wdk.NewSyncMap(),

		// TODO: Check when Satoshis field should be set
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sync state: %w", err)
	}

	return syncState, nil
}
