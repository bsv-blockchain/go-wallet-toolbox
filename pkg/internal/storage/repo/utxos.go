package repo

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

type UTXOs struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewUTXOs(db *gorm.DB, query *genquery.Query) *UTXOs {
	return &UTXOs{
		db:    db,
		query: query,
	}
}

func (u *UTXOs) FindNotReservedUTXOs(
	ctx context.Context,
	userID int,
	basketName string,
	page *queryopts.Paging,
	forbiddenOutputIDs []uint,
	includeSending bool,
) ([]*models.UserUTXO, error) {
	var result []*models.UserUTXO

	query := u.db.WithContext(ctx).Scopes(
		scopes.UserID(userID),
		scopes.BasketName(basketName),
		scopes.Paginate(page),
		notReserved(),
		outputNotIn(forbiddenOutputIDs),
	)

	statuses := []string{string(wdk.UTXOStatusMined), string(wdk.UTXOStatusUnproven)}
	if includeSending {
		statuses = append(statuses, string(wdk.UTXOStatusSending))
	}
	query.Where(u.query.UserUTXO.UTXOStatus.In(statuses...))

	err := query.Find(&result).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find not reserved UTXOs: %w", err)
	}
	return result, nil
}

func (u *UTXOs) CountUTXOs(ctx context.Context, userID int, basketName string) (int64, error) {
	count := int64(0)

	err := u.db.WithContext(ctx).
		Model(&models.UserUTXO{}).
		Scopes(scopes.UserID(userID), scopes.BasketName(basketName), notReserved()).
		Count(&count).Error

	return count, err
}

func (u *UTXOs) UnreserveUTXOsByTransactionID(ctx context.Context, transactionID uint) error {
	table := u.query.UserUTXO
	_, err := table.WithContext(ctx).
		Where(table.ReservedByID.Eq(transactionID)).
		Update(table.ReservedByID, nil)
	if err != nil {
		return fmt.Errorf("failed to unreserve UTXOs by transaction ID %d: %w", transactionID, err)
	}

	return nil
}

func notReserved() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("reserved_by_id IS NULL")
	}
}

func outputNotIn(forbiddenOutputIDs []uint) func(*gorm.DB) *gorm.DB {
	if len(forbiddenOutputIDs) == 0 {
		return func(db *gorm.DB) *gorm.DB {
			return db
		}
	}
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("output_id NOT IN ?", forbiddenOutputIDs)
	}
}
