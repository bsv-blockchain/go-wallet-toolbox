package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
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

	if err = backfillKnownTxBroadcastState(m.db.WithContext(ctx)); err != nil {
		return fmt.Errorf("failed to backfill known tx broadcast state: %w", err)
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

func backfillKnownTxBroadcastState(db *gorm.DB) error {
	return db.Model(&models.KnownTx{}).
		Where("status IN ?", []string{
			string(wdk.ProvenTxStatusUnmined),
			string(wdk.ProvenTxStatusCallback),
			string(wdk.ProvenTxStatusUnconfirmed),
			string(wdk.ProvenTxStatusCompleted),
			string(wdk.ProvenTxStatusReorg),
		}).
		Where("was_broadcast = ?", false).
		UpdateColumn("was_broadcast", true).
		Error
}
