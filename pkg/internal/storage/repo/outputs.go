package repo

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/genquery"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/txutils"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/must"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Outputs struct {
	db    *gorm.DB
	query *genquery.Query
}

func NewOutputs(db *gorm.DB, query *genquery.Query) *Outputs {
	return &Outputs{db: db, query: query}
}

func (o *Outputs) FindOutputs(ctx context.Context, outputIDs iter.Seq[uint]) ([]*entity.Output, error) {
	if seq.IsEmpty(outputIDs) {
		return nil, nil
	}

	idsClause := seq.Collect(outputIDs)

	var outputs []*models.Output
	err := o.db.WithContext(ctx).
		Model(models.Output{}).
		Preload("Transaction", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, tx_id")
		}).
		Where("id IN ?", idsClause).
		Find(&outputs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to find outputs: %w", err)
	}

	return slices.Map(outputs, o.mapModelToOutputEntity), nil
}

func (o *Outputs) FindOutputsByTransactionID(ctx context.Context, transactionID uint) ([]*entity.Output, error) {
	session := o.db.WithContext(ctx)

	var outputRows []*models.Output
	err := session.
		Model(models.Output{}).
		Where("transaction_id = ?", transactionID).
		Find(&outputRows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs for transactionID: %d: %w", transactionID, err)
	}

	return slices.Map(outputRows, o.mapModelToOutputEntity), nil
}

func (o *Outputs) ListAndCountOutputs(ctx context.Context, filter entity.ListOutputsFilter) ([]*entity.Output, int64, error) {
	var outputs []*models.Output
	var total int64

	if err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.
			Model(&models.Output{}).
			Where("user_id = ?", filter.UserID).
			Preload("Transaction", func(db *gorm.DB) *gorm.DB {
				return db.Select("id, tx_id")
			})

		var omitFields []string

		if !filter.IncludeLockingScripts {
			omitFields = append(omitFields, "locking_script")
		}

		if !filter.IncludeCustomInstructions {
			omitFields = append(omitFields, "custom_instructions")
		}

		if len(omitFields) > 0 {
			query = query.Omit(omitFields...)
		}

		if !filter.IncludeSpent {
			query = query.Where(o.query.Output.Spendable.Value(true))
		}

		if filter.Basket != "" {
			query = query.Where("basket_name = ?", filter.Basket)
		}

		if filter.IncludeTags {
			query = query.Preload("Tags")
		}

		if len(filter.Tags) > 0 {
			query = query.Scopes(o.tagFilterScope(tx, filter))
		}

		if err := query.Count(&total).Error; err != nil {
			return fmt.Errorf("count failed: %w", err)
		}

		if err := query.Limit(filter.Limit).Offset(filter.Offset).Order("bsv_outputs.id ASC").Find(&outputs).Error; err != nil {
			return fmt.Errorf("query failed: %w", err)
		}
		return nil
	}); err != nil {
		return nil, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return slices.Map(outputs, o.mapModelToOutputEntity), total, nil
}

func (o *Outputs) UnlinkOutputFromBasketByOutpoint(ctx context.Context, userID int, basketName *string, outpoint wdk.OutPoint) error {
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.Output{}).
			Select("id").
			Scopes(scopes.UserID(userID)).
			Where("vout = ?", outpoint.Vout).
			Where("transaction_id IN (?)",
				tx.Model(&models.Transaction{}).
					Select("id").
					Scopes(scopes.UserID(userID)).
					Where("tx_id = ?", outpoint.TxID),
			)

		if basketName != nil {
			query = query.Where("basket_name = ?", *basketName)
		}

		var output models.Output
		if err := query.First(&output).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				var basketMsg string
				if basketName != nil {
					basketMsg = fmt.Sprintf(" for basket: %s", *basketName)
				}
				return fmt.Errorf("no output found with vout %d and txid %s%s", outpoint.Vout, outpoint.TxID, basketMsg)
			}

			return fmt.Errorf("failed to fetch outputs for unlink: %w", err)
		}

		result := tx.Model(&models.Output{}).
			Where("id = ?", output.ID).
			Update("basket_name", nil)

		if result.Error != nil {
			return fmt.Errorf("failed to unlink output from basket: %w", result.Error)
		}

		err := tx.Delete(models.UserUTXO{}, "reserved_by_id IS NULL and output_id = ?", output.ID).Error
		if err != nil {
			return fmt.Errorf("failed to delete user utxo for output %d (it can be reserved): %w", output.ID, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to unlink output from basket: %w", err)
	}

	return nil
}

func (o *Outputs) FindOutputsByOutpoints(ctx context.Context, userID int, outpoints []wdk.OutPoint) ([]*entity.Output, error) {
	if len(outpoints) == 0 {
		return nil, nil
	}

	outpointStrings := slices.Map(outpoints, func(op wdk.OutPoint) []any {
		return []any{op.TxID, op.Vout}
	})
	outputTableName := o.query.Output.TableName()
	transactionTableName := o.query.Transaction.TableName()
	query := o.db.WithContext(ctx).Table(
		"(?) as out",
		o.db.Model(&models.Output{}).
			Select(fmt.Sprintf("%s.*, tx.tx_id as tx_id", outputTableName)).
			Joins(fmt.Sprintf("INNER JOIN %s tx ON tx.id = %s.transaction_id", transactionTableName, outputTableName)).
			Where(fmt.Sprintf("%s.user_id = ?", outputTableName), userID),
	).Where("(tx_id,vout) IN (?)", outpointStrings)

	type outputWithTxID struct {
		*models.Output
		TxID *string
	}

	var readModels []*outputWithTxID
	if err := query.Find(&readModels).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to fetch outputs: %w", err)
	}

	return slices.Map(readModels, func(readModel *outputWithTxID) *entity.Output {
		readModel.Output.Transaction = &models.Transaction{
			TxID: readModel.TxID,
		}
		return o.mapModelToOutputEntity(readModel.Output)
	}), nil
}

func (o *Outputs) FindOutput(ctx context.Context, userID int, outpoint wdk.OutPoint) (*entity.Output, error) {
	var output models.Output
	err := o.db.WithContext(ctx).
		Model(&models.Output{}).
		Scopes(scopes.UserID(userID)).
		Where("vout = ?", outpoint.Vout).
		Where("transaction_id IN (?)",
			o.db.Model(&models.Transaction{}).
				Select("id").
				Scopes(scopes.UserID(userID)).
				Where("tx_id = ?", outpoint.TxID),
		).
		First(&output).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find output: %w", err)
	}

	tableOutput := o.mapModelToOutputEntity(&output)
	tableOutput.TxID = &outpoint.TxID
	return tableOutput, nil
}

// FindInputsAndOutputsWithBaskets retrieves inputs and outputs for given transaction IDs, including basket information.
// It returns two maps: one for inputs keyed by SpentBy ID and another for outputs keyed by TransactionID.
// Each map contains slices of TableOutput, which include basket details if available.
func (o *Outputs) FindInputsAndOutputsWithBaskets(ctx context.Context, txIDs []uint, includeLockingScripts bool) (inputs map[uint][]*entity.Output, outputs map[uint][]*entity.Output, err error) {
	if len(txIDs) == 0 {
		return
	}

	query := o.db.WithContext(ctx).
		Model(&models.Output{}).
		Preload("Transaction", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, tx_id")
		}).
		Preload("Basket").
		Preload("Tags").
		Where("transaction_id IN ? OR spent_by IN ?", txIDs, txIDs)

	if !includeLockingScripts {
		query = query.Omit("locking_script")
	}

	var allOutputs []*models.Output
	if err := query.Find(&allOutputs).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to fetch inputs/outputs: %w", err)
	}

	inputMap := make(map[uint][]*entity.Output)
	outputMap := make(map[uint][]*entity.Output)

	for _, out := range allOutputs {
		tableOut := o.mapModelToOutputEntity(out)
		if out.SpentBy != nil {
			inputMap[*out.SpentBy] = append(inputMap[*out.SpentBy], tableOut)
		}
		outputMap[out.TransactionID] = append(outputMap[out.TransactionID], tableOut)
	}

	return inputMap, outputMap, nil
}

func (o *Outputs) SaveOutputs(ctx context.Context, outputs []*entity.Output) error {
	type outputWithTags struct {
		Output models.Output
		Tags   []any
	}

	modelsToStore := slices.Map(outputs, func(output *entity.Output) *outputWithTags {
		res := &outputWithTags{
			Output: models.Output{
				Model: gorm.Model{
					ID: output.ID,
				},
				UserID:             output.UserID,
				TransactionID:      output.TransactionID,
				SpentBy:            output.SpentBy,
				Vout:               output.Vout,
				Satoshis:           output.Satoshis,
				LockingScript:      output.LockingScript,
				CustomInstructions: output.CustomInstructions,
				DerivationPrefix:   output.DerivationPrefix,
				DerivationSuffix:   output.DerivationSuffix,
				BasketName:         output.BasketName,
				Spendable:          output.Spendable,
				Change:             output.Change,
				Description:        output.Description,
				ProvidedBy:         output.ProvidedBy,
				Purpose:            output.Purpose,
				Type:               output.Type,
				SenderIdentityKey:  output.SenderIdentityKey,
			},
			Tags: slices.Map(output.Tags, func(tag string) any {
				return &models.Tag{
					Name:   tag,
					UserID: output.UserID,
				}
			}),
		}

		if output.UserUTXO != nil {
			res.Output.UserUTXO = &models.UserUTXO{
				UserID:             output.UserUTXO.UserID,
				Satoshis:           output.UserUTXO.Satoshis,
				EstimatedInputSize: output.UserUTXO.EstimatedInputSize,
				UTXOStatus:         output.UserUTXO.Status,
			}
		}

		return res
	})

	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, model := range modelsToStore {
			err := tx.Save(&model.Output).Error
			if err != nil {
				return fmt.Errorf("failed to save output: %w", err)
			}

			association := tx.
				Model(&model.Output).
				Association("Tags")

			err = association.Replace(model.Tags...)
			if err != nil {
				return fmt.Errorf("failed to save current tags for output: %w", err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("db transaction failed: %w", err)
	}

	return nil
}

func (o *Outputs) MakeOutputsSpendable(ctx context.Context, txID string, utxoStatus wdk.UTXOStatus) error {
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var changeOutputs []*models.Output
		err := tx.Model(&models.Output{}).
			Select(
				o.query.Output.ID.ColumnName().String(),
				o.query.Output.BasketName.ColumnName().String(),
				o.query.Output.Satoshis.ColumnName().String(),
				o.query.Output.Type.ColumnName().String(),
				o.query.Output.UserID.ColumnName().String(),
			).
			Where("transaction_id IN (?)",
				o.db.Model(&models.Transaction{}).
					Select("id").
					Where("tx_id = ?", txID),
			).
			Where(o.query.Output.BasketName.IsNotNull()).
			Where(o.query.Output.Change.Is(true)).
			Where(o.query.Output.Satoshis.Gt(0)).
			Where(o.query.Output.SpentBy.IsNull()).
			Find(&changeOutputs).Error
		if err != nil {
			return fmt.Errorf("failed to find transaction outputs: %w", err)
		}

		if len(changeOutputs) == 0 {
			return nil
		}

		for _, output := range changeOutputs {
			err = tx.Model(&models.Output{}).
				Where("id = ?", output.ID).
				Updates(map[string]any{
					"spendable": true,
				}).Error
			if err != nil {
				return fmt.Errorf("failed to update output %d to spendable: %w", output.ID, err)
			}
		}

		newUTXOs := slices.Map(changeOutputs, func(output *models.Output) *models.UserUTXO {
			return &models.UserUTXO{
				UserID:             output.UserID,
				OutputID:           output.ID,
				BasketName:         *output.BasketName,
				Satoshis:           must.ConvertToUInt64(output.Satoshis),
				EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
				UTXOStatus:         utxoStatus,
			}
		})

		err = tx.
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(newUTXOs).Error
		if err != nil {
			return fmt.Errorf("failed to create new UTXOs: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to make outputs spendable: %w", err)
	}

	return nil
}

func (o *Outputs) mapModelToOutputEntity(model *models.Output) *entity.Output {
	output := &entity.Output{
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		ID:                 model.ID,
		UserID:             model.UserID,
		TransactionID:      model.TransactionID,
		BasketName:         model.BasketName,
		Spendable:          model.Spendable,
		Change:             model.Change,
		Description:        model.Description,
		Vout:               model.Vout,
		Satoshis:           model.Satoshis,
		ProvidedBy:         model.ProvidedBy,
		Purpose:            model.Purpose,
		Type:               model.Type,
		DerivationPrefix:   model.DerivationPrefix,
		DerivationSuffix:   model.DerivationSuffix,
		CustomInstructions: model.CustomInstructions,
		LockingScript:      model.LockingScript,
		SenderIdentityKey:  model.SenderIdentityKey,
		Tags:               slices.Map(model.Tags, func(tag *models.Tag) string { return tag.Name }),
	}
	if model.Transaction != nil && model.Transaction.TxID != nil {
		output.TxID = model.Transaction.TxID
	}
	return output
}

func (o *Outputs) tagFilterScope(tx *gorm.DB, filter entity.ListOutputsFilter) func(db *gorm.DB) *gorm.DB {
	return func(query *gorm.DB) *gorm.DB {
		subQuery := tx.Model(&models.OutputTag{}).
			Select("output_id").
			Where("tag_name IN ?", filter.Tags).
			Where("tag_user_id = ?", filter.UserID)

		if filter.TagsQueryMode == defs.QueryModeAll {
			subQuery = subQuery.Group("output_id").Having("COUNT(DISTINCT tag_name) = ?", len(filter.Tags))
		}

		return query.Where("id IN (?)", subQuery)
	}
}
