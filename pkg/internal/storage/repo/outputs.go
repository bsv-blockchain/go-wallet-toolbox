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
	"github.com/go-softwarelab/common/pkg/is"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
	"gorm.io/gorm"
)

type Outputs struct {
	db *gorm.DB
}

func NewOutputs(db *gorm.DB) *Outputs {
	return &Outputs{db: db}
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

	outpointStrings := slices.Map(outpoints, func(op wdk.OutPoint) string {
		return op.String()
	})

	outputTableName := genquery.Output.TableName()
	transactionTableName := genquery.Transaction.TableName()

	query := o.db.WithContext(ctx).Table(
		"(?) as out",
		o.db.Model(&models.Output{}).
			Select(fmt.Sprintf("%s.*, tx.tx_id as tx_id, CONCAT(tx.tx_id, '.', %s.vout) as outpoint", outputTableName, outputTableName)).
			Joins(fmt.Sprintf("INNER JOIN %s tx ON tx.id = %s.transaction_id", transactionTableName, outputTableName)).
			Where(fmt.Sprintf("%s.user_id = ?", outputTableName), userID),
	).Where("outpoint IN (?)", outpointStrings)

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

func (o *Outputs) SaveOutput(ctx context.Context, output *entity.Output) error {
	tags := slices.Map(output.Tags, func(tag string) any {
		return &models.Tag{
			Name:   tag,
			UserID: output.UserID,
		}
	})

	out := models.Output{
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
	}

	if out.Spendable && out.Change {
		if is.EmptyString(output.BasketName) {
			return fmt.Errorf("basket not provided for change output")
		}
		if out.Satoshis == 0 {
			return fmt.Errorf("change output with zero satoshis")
		}
		sats, err := to.UInt64(out.Satoshis)
		if err != nil {
			return fmt.Errorf("failed to convert satoshis to uint64: %w", err)
		}

		out.UserUTXO = &models.UserUTXO{
			UserID:             output.UserID,
			Satoshis:           sats,
			EstimatedInputSize: txutils.EstimatedInputSizeByType(wdk.OutputType(output.Type)),
		}
	}

	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Save(&out).Error
		if err != nil {
			return fmt.Errorf("failed to save output: %w", err)
		}

		association := tx.
			Model(&out).
			Association("Tags")

		err = association.Replace(tags...)
		if err != nil {
			return fmt.Errorf("failed to save current tags for output: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("db transaction failed: %w", err)
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
