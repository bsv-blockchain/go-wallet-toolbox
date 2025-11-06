package gormstorage

import (
	"context"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"gorm.io/gorm"
)

type storageQueries struct {
	ctx     context.Context
	query   *genquery.Query
	txQuery *genquery.QueryTx
}

func newStorageQueries(ctx context.Context, db *gorm.DB) *storageQueries {
	return &storageQueries{
		query: genquery.Use(db.WithContext(ctx)),
	}
}

func (i *storageQueries) getQuery() *genquery.Query {
	if i.txQuery != nil {
		return i.txQuery.Query
	}

	return i.query
}

func (i *storageQueries) Begin() {
	i.txQuery = i.query.Begin()
}

func (i *storageQueries) Rollback() error {
	if i.txQuery == nil {
		panic("transaction has not been started")
	}

	err := i.txQuery.Rollback()
	if err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}

	return nil
}

func (i *storageQueries) Commit() error {
	if i.txQuery == nil {
		panic("transaction has not been started")
	}

	err := i.txQuery.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (i *storageQueries) LiveHeaderExists(hash string) (bool, error) {
	count, err := i.getQuery().
		ChaintracksLiveHeader.
		Where(i.getQuery().ChaintracksLiveHeader.Hash.Eq(hash)).
		Count()
	if err != nil {
		return false, fmt.Errorf("failed to check live header existence: %w", err)
	}
	return count > 0, nil
}

func (i *storageQueries) GetLiveHeaderByHash(hash string) (*models.LiveBlockHeader, error) {
	model, err := i.getQuery().
		ChaintracksLiveHeader.
		Where(i.getQuery().ChaintracksLiveHeader.Hash.Eq(hash)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get live header by hash: %w", err)
	}

	return mapLiveHeader(model), nil
}
