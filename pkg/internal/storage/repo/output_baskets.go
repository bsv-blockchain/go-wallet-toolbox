package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type OutputBaskets struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewOutputBaskets(db *gorm.DB, query *genquery.Query) *OutputBaskets {
	return &OutputBaskets{db: db, query: query}
}

func (o *OutputBaskets) FindBasketByName(ctx context.Context, userID int, name string) (*entity.OutputBasket, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-OutputBasket-FindBasketByName", attribute.Int("UserID", userID), attribute.String("Name", name))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	outputBasket := &models.OutputBasket{}
	err = o.db.WithContext(ctx).
		Scopes(scopes.UserID(userID)).
		Where("name = ?", name).
		First(&outputBasket).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find output basket: %w", err)
	}

	return mapModelToEntityOutputBasket(outputBasket), nil
}

func (o *OutputBaskets) UpsertOutputBasket(ctx context.Context, userID int, basket wdk.BasketConfiguration) (isNew bool, err error) {
	ctx, span := tracing.StartTracing(ctx, "Repository-OutputBasket-UpsertOutputBasket", attribute.Int("UserID", userID), attribute.String("BasketName", string(basket.Name)))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	model := models.OutputBasket{
		UserID:                  userID,
		Name:                    string(basket.Name),
		NumberOfDesiredUTXOs:    basket.NumberOfDesiredUTXOs,
		MinimumDesiredUTXOValue: basket.MinimumDesiredUTXOValue,
	}

	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateTx := tx.Model(&models.OutputBasket{}).
			Scopes(scopes.UserID(userID)).
			Where("name = ?", basket.Name).
			Updates(map[string]interface{}{
				"number_of_desired_utxos":    basket.NumberOfDesiredUTXOs,
				"minimum_desired_utxo_value": basket.MinimumDesiredUTXOValue,
			})
		if updateTx.Error != nil {
			return fmt.Errorf("failed to update existing output basket: %w", updateTx.Error)
		}

		if updateTx.RowsAffected > 0 {
			return nil
		}

		err := tx.Create(&model).Error
		if err != nil {
			return fmt.Errorf("failed to create new output basket: %w", err)
		}

		isNew = true

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("transaction failed while upserting output basket: %w", err)
	}

	return isNew, nil
}

func mapModelToEntityOutputBasket(model *models.OutputBasket) *entity.OutputBasket {
	return &entity.OutputBasket{
		Name:                    model.Name,
		UserID:                  model.UserID,
		CreatedAt:               model.CreatedAt,
		UpdatedAt:               model.UpdatedAt,
		NumberOfDesiredUTXOs:    model.NumberOfDesiredUTXOs,
		MinimumDesiredUTXOValue: model.MinimumDesiredUTXOValue,
	}
}

func (o *OutputBaskets) AddOutputBasket(ctx context.Context, basket *entity.OutputBasket) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-OutputBasket-AddOutputBasket")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if basket == nil {
		return fmt.Errorf("output basket is nil")
	}
	model := &models.OutputBasket{
		UserID:                  basket.UserID,
		Name:                    basket.Name,
		NumberOfDesiredUTXOs:    basket.NumberOfDesiredUTXOs,
		MinimumDesiredUTXOValue: basket.MinimumDesiredUTXOValue,
	}
	if err := o.db.WithContext(ctx).Create(model).Error; err != nil {
		return fmt.Errorf("failed to add output basket: %w", err)
	}
	return nil
}

func (o *OutputBaskets) UpdateOutputBasket(ctx context.Context, spec *entity.OutputBasketUpdateSpecification) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-OutputBasket-UpdateOutputBasket", attribute.Int("UserID", spec.UserID))
	defer func() {
		tracing.EndTracing(span, err)
	}()

	err = o.db.WithContext(ctx).
		Model(&models.OutputBasket{}).
		Scopes(scopes.UserID(spec.UserID)).
		Where("name = ?", spec.Name).
		Updates(map[string]interface{}{
			"number_of_desired_utxos":    spec.NumberOfDesiredUTXOs,
			"minimum_desired_utxo_value": spec.MinimumDesiredUTXOValue,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to update output basket: %w", err)
	}
	return nil
}

func (o *OutputBaskets) FindOutputBaskets(ctx context.Context, spec *entity.OutputBasketReadSpecification, opts ...queryopts.Options) ([]*entity.OutputBasket, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-OutputBasket-FindOutputBaskets")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &o.query.OutputBasket
	modelsList, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(o.conditionsBySpec(spec)...).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find output baskets: %w", err)
	}
	// map to entities
	result := make([]*entity.OutputBasket, len(modelsList))
	for i, m := range modelsList {
		result[i] = mapModelToEntityOutputBasket(m)
	}
	return result, nil
}

func (o *OutputBaskets) CountOutputBaskets(ctx context.Context, spec *entity.OutputBasketReadSpecification, opts ...queryopts.Options) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-OutputBasket-CountOutputBaskets")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &o.query.OutputBasket
	count, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(o.conditionsBySpec(spec)...).
		Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count output baskets: %w", err)
	}
	return count, nil
}

func (o *OutputBaskets) conditionsBySpec(spec *entity.OutputBasketReadSpecification) []gen.Condition {
	if spec == nil {
		return nil
	}
	table := &o.query.OutputBasket
	var conditions []gen.Condition
	if spec.UserID != nil {
		conditions = append(conditions, cmpCondition(table.UserID, spec.UserID))
	}
	if spec.Name != nil {
		conditions = append(conditions, cmpCondition(table.Name, spec.Name))
	}
	if spec.NumberOfDesiredUTXOs != nil {
		conditions = append(conditions, cmpCondition(table.NumberOfDesiredUTXOs, spec.NumberOfDesiredUTXOs))
	}
	if spec.MinimumDesiredUTXOValue != nil {
		conditions = append(conditions, cmpCondition(table.MinimumDesiredUTXOValue, spec.MinimumDesiredUTXOValue))
	}

	return conditions
}
