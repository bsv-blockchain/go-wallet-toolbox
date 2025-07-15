package repo

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
)

type TxNotes struct {
	db *gorm.DB
}

func NewTxEvents(db *gorm.DB) *TxNotes {
	return &TxNotes{db: db}
}

func addTxNote(tx *gorm.DB, txNote *entity.TxNote) error {
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
