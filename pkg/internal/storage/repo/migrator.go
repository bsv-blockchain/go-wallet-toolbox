package repo

import (
	"context"
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"gorm.io/gorm"
)

type Migrator struct {
	db *gorm.DB
}

func NewMigrator(db *gorm.DB) *Migrator {
	return &Migrator{db: db}
}

func (m *Migrator) Migrate(ctx context.Context) error {
	err := m.db.WithContext(ctx).AutoMigrate(
		models.Setting{},
		models.User{},
		models.OutputBasket{},
		models.CertificateField{},
		models.Certificate{},
		models.UserUTXO{},
		models.Transaction{},
		models.Output{},
		models.KnownTx{},
		models.Label{},
		models.TransactionLabel{},
		models.NumericIDLookup{},
		models.SyncState{},
		models.KeyValue{},
		models.Tag{},
		models.OutputTags{},
		models.Commission{},
		models.TxNote{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate settings: %w", err)
	}

	err = m.db.SetupJoinTable(&models.Transaction{}, "Labels", &models.TransactionLabel{})
	if err != nil {
		return fmt.Errorf("failed to setup join table for Transaction and Labels: %w", err)
	}

	return nil
}
