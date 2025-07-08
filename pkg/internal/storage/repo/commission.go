package repo

import (
	"context"
	"errors"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gen"
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
	commission, err := genquery.Commission.WithContext(ctx).GetByUserIDAndTransactionID(userID, transactionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find commission: %w", err)
	}

	return mapModelToEntityCommission(&commission), nil
}

func (c *Commission) FindCommissions(ctx context.Context, spec *entity.CommissionSpecification, opts ...queryopts.Options) ([]*entity.Commission, error) {
	table := genquery.Commission

	commissions, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(c.conditionsBySpec(spec)...).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find commissions: %w", err)
	}

	return slices.Map(commissions, mapModelToEntityCommission), nil
}

func (c *Commission) conditionsBySpec(spec *entity.CommissionSpecification) []gen.Condition {
	if spec == nil {
		return []gen.Condition{}
	}

	if spec.ID != nil {
		return []gen.Condition{genquery.Commission.ID.Eq(*spec.ID)}
	}

	var conditions []gen.Condition

	if spec.IsRedeemed != nil {
		conditions = append(conditions, genquery.Commission.IsRedeemed.Is(*spec.IsRedeemed))
	}

	if spec.Satoshis != nil {
		conditions = append(conditions, cmpCondition(genquery.Commission.Satoshis, spec.Satoshis.Cmp, spec.Satoshis.Value))
	}

	return conditions
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
