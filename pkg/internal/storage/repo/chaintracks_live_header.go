package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
)

type ChaintracksLiveHeader struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewChaintracksLiveHeader(db *gorm.DB, query *genquery.Query) *ChaintracksLiveHeader {
	return &ChaintracksLiveHeader{db: db, query: query}
}

func (ct *ChaintracksLiveHeader) DBTransaction(ctx context.Context, action TransactionFunc) error {
	return newDBTransaction(ctx, ct.db, action)
}

func (ct *ChaintracksLiveHeader) session(ctx context.Context) *gorm.DB {
	if tx, ok := dbTxFromContext(ctx); ok {
		return tx
	}
	return ct.db.WithContext(ctx)
}

func (ct *ChaintracksLiveHeader) LiveHeaderExists(ctx context.Context, hash string) (bool, error) {
	var count int64
	err := ct.session(ctx).
		Model(&models.ChaintracksLiveHeader{}).
		Where("hash = ?", hash).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check live header existence: %w", err)
	}
	return count > 0, nil
}

func (ct *ChaintracksLiveHeader) GetLiveHeaderByHash(ctx context.Context, hash string) (*models.ChaintracksLiveHeader, error) {
	var header models.ChaintracksLiveHeader
	err := ct.session(ctx).
		Where("hash = ?", hash).
		First(&header).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get live header by hash: %w", err)
	}
	return &header, nil
}

func (ct *ChaintracksLiveHeader) GetActiveChainTip(ctx context.Context) (*models.ChaintracksLiveHeader, error) {
	var header models.ChaintracksLiveHeader
	err := ct.session(ctx).
		Where("is_active = ?", true).
		Where("is_chain_tip = ?", true).
		First(&header).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active chain tip: %w", err)
	}
	return &header, nil
}
