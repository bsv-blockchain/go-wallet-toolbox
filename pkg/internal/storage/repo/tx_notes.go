package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/queryopts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/tracing"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type TxNotes struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewTxNotes(db *gorm.DB, query *genquery.Query) *TxNotes {
	return &TxNotes{db: db, query: query}
}

func (r *TxNotes) FindTxNotes(ctx context.Context, spec *entity.TxNoteReadSpecification, opts ...queryopts.Options) ([]*entity.TxNotes, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-TxNotes-FindTxNotes")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &r.query.TxNote

	records, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(r.conditionsBySpec(spec)...).
		Find()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tx notes: %w", err)
	}

	return slices.Map(records, func(model *models.TxNote) *entity.TxNotes {
		return &entity.TxNotes{
			ID:        model.ID,
			CreatedAt: model.CreatedAt,
			DeletedAt: func() *time.Time {
				if model.DeletedAt.Valid {
					return &model.DeletedAt.Time
				}
				return nil
			}(),
			TxID:       model.TxID,
			UserID:     model.UserID,
			What:       model.What,
			Attributes: model.Attributes,
		}
	}), nil
}

func (r *TxNotes) CountTxNotes(ctx context.Context, spec *entity.TxNoteReadSpecification, opts ...queryopts.Options) (int64, error) {
	var err error
	ctx, span := tracing.StartTracing(ctx, "Repository-TxNotes-CountTxNotes")
	defer func() {
		tracing.EndTracing(span, err)
	}()

	table := &r.query.TxNote

	count, err := table.WithContext(ctx).
		Scopes(scopes.FromQueryOptsForGen(table, opts)...).
		Where(r.conditionsBySpec(spec)...).
		Count()
	if err != nil {
		return 0, fmt.Errorf("failed to count tx notes: %w", err)
	}

	return count, nil
}

func (r *TxNotes) conditionsBySpec(spec *entity.TxNoteReadSpecification) []gen.Condition {
	if spec == nil {
		return nil
	}

	table := r.query.TxNote

	if spec.TxID != nil {
		return []gen.Condition{table.TxID.Eq(*spec.TxID)}
	}

	var conditions []gen.Condition
	if spec.UserID != nil {
		conditions = append(conditions, cmpCondition(table.UserID, spec.UserID))
	}
	if spec.What != nil {
		conditions = append(conditions, cmpCondition(table.What, spec.What))
	}
	if spec.CreatedAt != nil {
		conditions = append(conditions, cmpTimeCondition(table.CreatedAt, spec.CreatedAt))
	}

	return conditions
}

func addTxNote(tx *gorm.DB, txNote *entity.TxHistoryNote) error {
	model := models.TxNote{
		TxID:       txNote.TxID,
		UserID:     txNote.UserID,
		What:       txNote.What,
		Attributes: txNote.Attributes,
	}

	if err := tx.Create(&model).Error; err != nil {
		return fmt.Errorf("failed to create transaction history note: %w", err)
	}

	return nil
}

func addTxNotes(tx *gorm.DB, txNotes []*entity.TxHistoryNote) error {
	if len(txNotes) == 0 {
		return nil
	}

	modelsToAdd := slices.Map(txNotes, func(note *entity.TxHistoryNote) *models.TxNote {
		return &models.TxNote{
			TxID:       note.TxID,
			UserID:     note.UserID,
			What:       note.What,
			Attributes: note.Attributes,
		}
	})

	if err := tx.Create(&modelsToAdd).Error; err != nil {
		return fmt.Errorf("failed to create transaction history notes: %w", err)
	}

	return nil
}

func mapModelToEntityTxNote(model *models.TxNote) *entity.TxHistoryNote {
	if model == nil {
		return nil
	}

	return &entity.TxHistoryNote{
		TxID: model.TxID,
		HistoryNote: wdk.HistoryNote{
			When:       model.CreatedAt,
			UserID:     model.UserID,
			What:       model.What,
			Attributes: model.Attributes,
		},
	}
}
