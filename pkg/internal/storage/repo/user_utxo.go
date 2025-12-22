package repo

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type UserUTXOs struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewUserUTXOs(db *gorm.DB, query *genquery.Query) *UserUTXOs {
	return &UserUTXOs{db: db, query: query}
}

func (r *UserUTXOs) Add(ctx context.Context, utxo *entity.UserUTXO) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-UserUtxo-Add")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	if utxo == nil {
		err = fmt.Errorf("utxo cannot be nil")
		return err
	}
	model := &models.UserUTXO{
		UserID:             utxo.UserID,
		OutputID:           utxo.OutputID,
		BasketName:         utxo.BasketName,
		Satoshis:           utxo.Satoshis,
		EstimatedInputSize: utxo.EstimatedInputSize,
		CreatedAt:          utxo.CreatedAt,
		ReservedByID:       utxo.ReservedByID,
		UTXOStatus:         utxo.Status,
	}
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *UserUTXOs) Update(ctx context.Context, spec *entity.UserUTXOUpdateSpecification) error {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-UserUtxo-Update")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &r.query.UserUTXO
	updates := map[string]any{}

	if spec.ReservedByID != nil {
		updates[table.ReservedByID.ColumnName().String()] = *spec.ReservedByID
	}
	if spec.Status != nil {
		updates[table.UTXOStatus.ColumnName().String()] = string(*spec.Status)
	}
	if spec.BasketName != nil {
		updates[table.BasketName.ColumnName().String()] = *spec.BasketName
	}
	if spec.Satoshis != nil {
		updates[table.Satoshis.ColumnName().String()] = *spec.Satoshis
	}
	if spec.EstimatedInputSize != nil {
		updates[table.EstimatedInputSize.ColumnName().String()] = *spec.EstimatedInputSize
	}
	if spec.UserID != nil {
		updates[table.UserID.ColumnName().String()] = *spec.UserID
	}

	if len(updates) == 0 {
		return nil
	}

	_, err = table.WithContext(ctx).
		Where(table.OutputID.Eq(spec.OutputID)).
		Updates(updates)
	if err != nil {
		return fmt.Errorf("failed to update user utxo: %w", err)
	}
	return nil
}

func (r *UserUTXOs) Find(ctx context.Context, spec *entity.UserUTXOReadSpecification, opts ...queryopts.Options) ([]*entity.UserUTXO, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-UserUtxo-Find")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &r.query.UserUTXO
	rows, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(r.conditionsBySpec(spec)...).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to find user utxos: %w", err)
	}
	return slices.Map(rows, mapUserUTXOModelToEntity), nil
}

func (r *UserUTXOs) Count(ctx context.Context, spec *entity.UserUTXOReadSpecification, opts ...queryopts.Options) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-UserUtxo-Count")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &r.query.UserUTXO

	count, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(r.conditionsBySpec(spec)...).
		Count()

	if err != nil {
		return 0, fmt.Errorf("failed to count user utxos: %w", err)
	}
	return count, nil
}

func (r *UserUTXOs) conditionsBySpec(spec *entity.UserUTXOReadSpecification) []gen.Condition {
	if spec == nil {
		return nil
	}
	table := &r.query.UserUTXO
	var conds []gen.Condition

	if spec.UserID != nil {
		conds = append(conds, table.UserID.Eq(*spec.UserID))
	}
	if spec.OutputID != nil {
		conds = append(conds, cmpCondition(table.OutputID, spec.OutputID))
	}
	if spec.BasketName != nil {
		conds = append(conds, cmpCondition(table.BasketName, spec.BasketName))
	}
	if spec.Status != nil {
		conds = append(conds, cmpCondition(table.UTXOStatus, spec.Status.ToStringComparable()))
	}
	if spec.Satoshis != nil {
		conds = append(conds, cmpCondition(table.Satoshis, spec.Satoshis))
	}
	if spec.EstimatedInputSize != nil {
		conds = append(conds, cmpCondition(table.EstimatedInputSize, spec.EstimatedInputSize))
	}
	if spec.ReservedByID != nil {
		conds = append(conds, cmpCondition(table.ReservedByID, spec.ReservedByID))
	}

	return conds
}

func mapUserUTXOModelToEntity(model *models.UserUTXO) *entity.UserUTXO {
	return &entity.UserUTXO{
		UserID:             model.UserID,
		OutputID:           model.OutputID,
		BasketName:         model.BasketName,
		Satoshis:           model.Satoshis,
		EstimatedInputSize: model.EstimatedInputSize,
		CreatedAt:          model.CreatedAt,
		ReservedByID:       model.ReservedByID,
		Status:             model.UTXOStatus,
	}
}
