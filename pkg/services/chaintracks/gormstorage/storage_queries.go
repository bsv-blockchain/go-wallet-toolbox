package gormstorage

import (
	"context"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	dbmodels "github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
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
	table := i.getQuery().ChaintracksLiveHeader
	count, err := table.
		Where(table.Hash.Eq(hash)).
		Count()
	if err != nil {
		return false, fmt.Errorf("failed to check live header existence: %w", err)
	}
	return count > 0, nil
}

func (i *storageQueries) GetLiveHeaderByHash(hash string) (*models.LiveBlockHeader, error) {
	table := i.getQuery().ChaintracksLiveHeader
	model, err := table.
		Where(table.Hash.Eq(hash)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get live header by hash: %w", err)
	}

	return mapLiveHeader(model), nil
}

func (i *storageQueries) GetActiveTipLiveHeader() (*models.LiveBlockHeader, error) {
	table := i.getQuery().ChaintracksLiveHeader
	model, err := table.
		Where(table.IsActive.Is(true)).
		Where(table.IsChainTip.Is(true)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active tip live header: %w", err)
	}
	return mapLiveHeader(model), nil
}

func (i *storageQueries) SetChainTipByID(id uint, isChainTip bool) error {
	table := i.getQuery().ChaintracksLiveHeader
	_, err := table.
		Where(table.HeaderID.Eq(id)).
		UpdateColumn(table.IsChainTip, isChainTip)
	if err != nil {
		return fmt.Errorf("failed to set chain tip by ID: %w", err)
	}
	return nil
}

func (i *storageQueries) InsertNewLiveHeader(header *models.LiveBlockHeader) error {
	table := i.getQuery().ChaintracksLiveHeader
	err := table.Create(&dbmodels.ChaintracksLiveHeader{
		PreviousHeaderID: header.PreviousHeaderID,
		PreviousHash:     header.PreviousHash,
		Height:           header.Height,
		IsActive:         header.IsActive,
		IsChainTip:       header.IsChainTip,
		Hash:             header.Hash,
		ChainWork:        header.ChainWork,
		Version:          header.Version,
		MerkleRoot:       header.MerkleRoot,
		Time:             header.Time,
		Bits:             header.Bits,
		Nonce:            header.Nonce,
	})
	if err != nil {
		return fmt.Errorf("failed to insert new live header: %w", err)
	}
	return nil
}

func (i *storageQueries) CountLiveHeaders() (int64, error) {
	table := i.getQuery().ChaintracksLiveHeader
	count, err := table.Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count live headers: %w", err)
	}
	return count, nil
}
