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
		models.OutputTag{},
		models.Commission{},
		models.TxNote{},
		models.ChaintracksLiveHeader{},
		models.ChaintracksBulkFile{},
	)
	if err != nil {
		return fmt.Errorf("failed to auto migrate models: %w", err)
	}

	err = m.db.SetupJoinTable(&models.Transaction{}, "Labels", &models.TransactionLabel{})
	if err != nil {
		return fmt.Errorf("failed to setup join table for Transaction and Labels: %w", err)
	}

	err = m.db.SetupJoinTable(&models.Output{}, "Tags", &models.OutputTag{})
	if err != nil {
		return fmt.Errorf("failed to setup join table for Output and Tags: %w", err)
	}

	return nil
}
