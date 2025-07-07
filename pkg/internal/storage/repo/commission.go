package repo

import (
	"context"
	"errors"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/slices"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var commissionFieldToColumn = newFieldToColumn(entity.CommissionFieldNames, models.CommissionColumnNames).QueryOptsModifier()

type Commission struct {
	db *gorm.DB
}

func NewCommission(db *gorm.DB) *Commission {
	return &Commission{db: db}
}

func (c *Commission) AddCommission(ctx context.Context, commission *entity.Commission) error {
	if commission == nil {
		return nil
	}

	model := &models.Commission{
		UserID:        commission.UserID,
		TransactionID: commission.TransactionID,
		Satoshis:      commission.Satoshis,
		KeyOffset:     commission.KeyOffset,
		IsRedeemed:    commission.IsRedeemed,
		LockingScript: commission.LockingScript,
	}

	err := c.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(model).Error
	if err != nil {
		return fmt.Errorf("failed to add commission: %w", err)
	}

	return nil
}

func (c *Commission) FindCommission(ctx context.Context, userID int, transactionID uint) (*entity.Commission, error) {
	commission := &models.Commission{}
	err := c.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Where("transaction_id = ?", transactionID).
		First(commission).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find commission: %w", err)
	}

	return mapModelToEntityCommission(commission), nil
}

func (c *Commission) FindCommissions(ctx context.Context, opts ...queryopts.Options) ([]*entity.Commission, error) {
	queryopts.ModifyOptions(opts, commissionFieldToColumn)

	var commissions []*models.Commission
	err := c.db.WithContext(ctx).
		Scopes(scopes.FromQueryOpts(opts)...).
		Model(&models.Commission{}).
		Find(&commissions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find commissions: %w", err)
	}

	return slices.Map(commissions, mapModelToEntityCommission), nil
}

func mapModelToEntityCommission(model *models.Commission) *entity.Commission {
	if model == nil {
		return nil
	}

	return &entity.Commission{
		ID:            model.ID,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		UserID:        model.UserID,
		TransactionID: model.TransactionID,
		Satoshis:      model.Satoshis,
		KeyOffset:     model.KeyOffset,
		IsRedeemed:    model.IsRedeemed,
		LockingScript: model.LockingScript,
	}
}
