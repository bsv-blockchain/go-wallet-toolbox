package repo

import (
	"context"
	"fmt"
	"iter"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/database/models"
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

func (o *Outputs) FindInputsAndOutputsOfTransaction(ctx context.Context, transactionID uint) (inputs []*wdk.TableOutput, outputs []*wdk.TableOutput, err error) {
	session := o.db.WithContext(ctx)

	var outputRows []*models.Output
	err = session.
		Model(models.Output{}).
		Where("transaction_id = ?", transactionID).
		Find(&outputRows).Error
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find outputs for transactionID: %d: %w", transactionID, err)
	}

	var inputRows []*models.Output
	err = session.
		Model(models.Output{}).
		Where("spent_by = ?", transactionID).
		Find(&inputRows).Error
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find inputs for transactionID: %d: %w", transactionID, err)
	}

	inputs = slices.Map(inputRows, o.mapModelToTableOutput)
	outputs = slices.Map(outputRows, o.mapModelToTableOutput)
	return
}

func (o *Outputs) mapModelToTableOutput(model *models.Output) *wdk.TableOutput {
	output := &wdk.TableOutput{
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
		OutputID:           model.ID,
		UserID:             model.UserID,
		TransactionID:      model.TransactionID,
		BasketID:           model.BasketID,
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
