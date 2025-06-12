package sync

import (
	"context"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/logging"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"log/slog"
	"time"
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

func (f *findOrInsertSyncState) FindOrInsertSyncState(_ context.Context, auth wdk.AuthID, storageIdentityKey, storageName string) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
	reference, err := f.random.Base64(12)
	if err != nil {
		return nil, fmt.Errorf("failed to generate reference number: %w", err)
	}

	syncMapJSON, err := wdk.NewSyncMap().JSON()
	if err != nil {
		return nil, fmt.Errorf("failed to create new sync map: %w", err)
	}

	return &wdk.FindOrInsertSyncStateAuthResponse{
		SyncState: &wdk.TableSyncState{
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
			SyncStateID:        0,
			UserID:             *auth.UserID,       // NOTE: This is the userID of a storage that wants to do the backup
			StorageIdentityKey: storageIdentityKey, // NOTE: This is the storage that wants to do the backup
			StorageName:        storageName,
			Status:             wdk.SyncStatusUnknown,
			Init:               false,
			RefNum:             reference,
			SyncMap:            string(syncMapJSON),
			When:               nil,
			Satoshis:           nil,
			ErrorLocal:         nil,
			ErrorOther:         nil,
		},
		IsNew: true,
	}, nil
}
