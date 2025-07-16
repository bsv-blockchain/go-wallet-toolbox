package repo

import (
	"fmt"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
)

func addTxNote(tx *gorm.DB, txNote *entity.TxHistoryNote) error {
	note := txNote.Content
	model := models.TxNote{
		TxID:       txNote.TxID,
		UserID:     note.UserID,
		What:       note.What,
		Attributes: note.Attributes,
	}

	if err := tx.Create(&model).Error; err != nil {
		return fmt.Errorf("failed to create transaction history note: %w", err)
	}

	return nil
}

func mapModelToEntityTxNote(model *models.TxNote) *entity.TxHistoryNote {
	if model == nil {
		return nil
	}

	return &entity.TxHistoryNote{
		TxID: model.TxID,
		Content: wdk.HistoryNote{
			When:       model.CreatedAt,
			UserID:     model.UserID,
			What:       model.What,
			Attributes: model.Attributes,
		},
	}
}
