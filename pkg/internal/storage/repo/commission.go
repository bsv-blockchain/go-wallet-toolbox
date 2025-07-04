package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

	return &entity.Commission{
		UserID:        commission.UserID,
		TransactionID: commission.TransactionID,
		Satoshis:      commission.Satoshis,
		KeyOffset:     commission.KeyOffset,
		IsRedeemed:    commission.IsRedeemed,
		LockingScript: commission.LockingScript,
	}, nil
}
