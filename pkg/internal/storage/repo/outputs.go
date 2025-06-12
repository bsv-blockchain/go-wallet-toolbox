package repo

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/scopes"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/seq"
	"github.com/go-softwarelab/common/pkg/slices"
	"gorm.io/gorm"
)

type Outputs struct {
	db *gorm.DB
}

func NewOutputs(db *gorm.DB) *Outputs {
	return &Outputs{db: db}
}

func (o *Outputs) FindOutputs(ctx context.Context, outputIDs iter.Seq[uint]) ([]*wdk.TableOutput, error) {
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

	return slices.Map(outputs, o.mapModelToTableOutput), nil
}

func (o *Outputs) FindOutputsByTransactionID(ctx context.Context, transactionID uint) ([]*wdk.TableOutput, error) {
	session := o.db.WithContext(ctx)

	var outputRows []*models.Output
	err := session.
		Model(models.Output{}).
		Where("transaction_id = ?", transactionID).
		Find(&outputRows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find outputs for transactionID: %d: %w", transactionID, err)
	}

	return slices.Map(outputRows, o.mapModelToTableOutput), nil
}

func (o *Outputs) mapModelToTableOutput(model *models.Output) *wdk.TableOutput {
	output := &wdk.TableOutput{
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		OutputID:           model.ID,
		UserID:             model.UserID,
		TransactionID:      model.TransactionID,
		BasketName:         model.BasketName,
		Spendable:          model.Spendable,
		Change:             model.Change,
		OutputDescription:  model.Description,
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
	}
	if model.Transaction != nil {
		output.TxID = model.Transaction.TxID
	}
	return output
}

func (o *Outputs) ListAndCountOutputs(ctx context.Context, filter entity.ListOutputsFilter) ([]*wdk.TableOutput, int64, error) {
	var outputs []*models.Output
	var total int64

	if err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.
			Model(&models.Output{}).
			Where("user_id = ?", filter.UserID)

		if filter.Basket != "" {
			query = query.Where("basket_name = ?", filter.Basket)
		}

		if filter.IncludeTXID {
			query = query.Preload("Transaction", func(db *gorm.DB) *gorm.DB {
				return db.Select("id, tx_id")
			})
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

	return slices.Map(outputs, o.mapModelToTableOutput), total, nil
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

func (o *Outputs) FindOutput(ctx context.Context, userID int, outpoint wdk.OutPoint) (*wdk.TableOutput, error) {
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

	tableOutput := o.mapModelToTableOutput(&output)
	tableOutput.TxID = &outpoint.TxID
	return tableOutput, nil
}

// FindInputsAndOutputsWithBaskets retrieves inputs and outputs for given transaction IDs, including basket information.
// It returns two maps: one for inputs keyed by SpentBy ID and another for outputs keyed by TransactionID.
// Each map contains slices of TableOutput, which include basket details if available.
func (o *Outputs) FindInputsAndOutputsWithBaskets(ctx context.Context, txIDs []uint, includeLockingScripts bool) (inputs map[uint][]*wdk.TableOutput, outputs map[uint][]*wdk.TableOutput, err error) {
	if len(txIDs) == 0 {
		return
	}

	query := o.db.WithContext(ctx).
		Model(&models.Output{}).
		Preload("Basket").
		Where("transaction_id IN ? OR spent_by IN ?", txIDs, txIDs)

	if !includeLockingScripts {
		query = query.Omit("locking_script")
	}

	var allOutputs []*models.Output
	if err := query.Find(&allOutputs).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to fetch inputs/outputs: %w", err)
	}

	inputMap := make(map[uint][]*wdk.TableOutput)
	outputMap := make(map[uint][]*wdk.TableOutput)

	for _, out := range allOutputs {
		tableOut := o.mapModelToTableOutput(out)
		if out.SpentBy != nil {
			inputMap[*out.SpentBy] = append(inputMap[*out.SpentBy], tableOut)
		}
		outputMap[out.TransactionID] = append(outputMap[out.TransactionID], tableOut)
	}

	return inputMap, outputMap, nil
}
