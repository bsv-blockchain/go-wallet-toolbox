package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
)

type KeyValue struct {
	db *gorm.DB
}

func NewKeyValue(db *gorm.DB) *KeyValue {
	return &KeyValue{db: db}
}

func (kv *KeyValue) Get(ctx context.Context, key string) ([]byte, bool, error) {
	var model models.KeyValue
	err := kv.db.WithContext(ctx).
		Where("key = ?", key).
		Select("value").
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil // Key not found
		}
		return nil, false, fmt.Errorf("failed to get key %s: %w", key, err)
	}

	return model.Value, true, nil
}

func (kv *KeyValue) Set(ctx context.Context, key string, value []byte) error {
	err := kv.db.WithContext(ctx).Model(&models.KeyValue{}).
		Where("key = ?", key).
		Save(&models.KeyValue{
			Key:   key,
			Value: value,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to set value for key %s: %w", key, err)
	}

	return nil
}
