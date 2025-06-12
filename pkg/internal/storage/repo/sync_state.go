package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/satoshi"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
)

type SyncState struct {
	db *gorm.DB
}

func NewSyncState(db *gorm.DB) *SyncState {
	return &SyncState{
		db: db,
	}
}

func (s *SyncState) FindSyncState(ctx context.Context, userID int, storageIdentityKey string, storageName string) (*entity.SyncState, error) {
	var model models.SyncState
	err := s.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Where("storage_identity_key = ? AND storage_name = ?", storageIdentityKey, storageName).
		First(&model).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find sync state: %w", err)
	}

	return mapModelToSyncStateEntity(model)
}

func (s *SyncState) CreateSyncState(ctx context.Context, syncState *entity.SyncState) (*entity.SyncState, error) {
	syncMapJSON, err := syncState.SyncMap.JSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sync map: %w", err)
	}

	model := models.SyncState{
		UserID:             syncState.UserID,
		StorageIdentityKey: syncState.StorageIdentityKey,
		StorageName:        syncState.StorageName,
		Status:             syncState.Status,
		RefNum:             syncState.Reference,
		SyncMap:            syncMapJSON,
		When:               syncState.When,
	}

	if syncState.Satoshis != nil {
		model.Satoshis = to.Ptr(syncState.Satoshis.Int64())
	}

	err = s.db.WithContext(ctx).Create(&model).Error
	if err != nil {
		return nil, fmt.Errorf("failed to create sync state: %w", err)
	}

	return mapModelToSyncStateEntity(model)
}

func mapModelToSyncStateEntity(model models.SyncState) (*entity.SyncState, error) {
	syncMap, err := wdk.NewSyncMapFromJSON(model.SyncMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal sync map: %w", err)
	}

	entityModel := &entity.SyncState{
		ID:                 model.ID,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		UserID:             model.UserID,
		StorageIdentityKey: model.StorageIdentityKey,
		StorageName:        model.StorageName,
		Status:             model.Status,
		Reference:          model.RefNum,
		SyncMap:            syncMap,
		When:               model.When,
	}

	if model.Satoshis != nil {
		entityModel.Satoshis = to.Ptr(satoshi.MustFrom(*model.Satoshis))
	}

	return entityModel, nil
}
