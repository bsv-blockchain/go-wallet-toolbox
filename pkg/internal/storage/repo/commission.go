package repo

import (
	"context"
	"errors"
	"fmt"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models/query"
	"gorm.io/gen"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/go-softwarelab/common/pkg/slices"
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

	return mapModelToEntityCommission(commission), nil
}

func (c *Commission) FindCommissions(ctx context.Context, spec *entity.CommissionSpecification, opts ...queryopts.Options) ([]*entity.Commission, error) {
	var conditions []gen.Condition
	if spec != nil {
		if spec.ID != nil {
			conditions = append(conditions, query.Commission.ID.Eq(*spec.ID))
		} else {
			if spec.IsRedeemed != nil {
				conditions = append(conditions, query.Commission.IsRedeemed.Is(*spec.IsRedeemed))
			}
			if spec.Satoshis != nil {
				switch spec.Satoshis.Cmp {
				case entity.Equal:
					conditions = append(conditions, query.Commission.Satoshis.Eq(spec.Satoshis.Value))
				case entity.GreaterThan:
					conditions = append(conditions, query.Commission.Satoshis.Gt(spec.Satoshis.Value))
				case entity.LessThan:
					conditions = append(conditions, query.Commission.Satoshis.Lt(spec.Satoshis.Value))
				case entity.GreaterThanOrEqual:
					conditions = append(conditions, query.Commission.Satoshis.Gte(spec.Satoshis.Value))
				case entity.LessThanOrEqual:
					conditions = append(conditions, query.Commission.Satoshis.Lte(spec.Satoshis.Value))
				case entity.NotEqual:
					conditions = append(conditions, query.Commission.Satoshis.Neq(spec.Satoshis.Value))
				default:
					return nil, fmt.Errorf("unsupported comparison operator: %s", spec.Satoshis.Cmp)
				}
			}
		}
	}

	commissions, err := query.Commission.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(query.Commission, opts)...).
		Where(conditions...).
		Find()
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
