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
		models.Transaction{},
		models.Output{},
		models.ProvenTxReq{},
		models.ProvenTx{},
		models.TxLabel{},
		models.TxLabelsMap{},
		models.SyncState{},
		models.KeyValue{},
		models.OutputTag{},
		models.OutputTagsMap{},
		models.Commission{},
		models.MonitorEvent{},
		models.ChaintracksLiveHeader{},
		models.ChaintracksBulkFile{},
	)
	if err != nil {
		return fmt.Errorf("failed to auto migrate models: %w", err)
	}

	if err = backfillKnownTxBroadcastState(m.db.WithContext(ctx)); err != nil {
		return fmt.Errorf("failed to backfill known tx broadcast state: %w", err)
	}

	return nil
}

func backfillKnownTxBroadcastState(db *gorm.DB) error {
	return db.Model(&models.ProvenTxReq{}).
		Where("status IN ?", []string{
			string(wdk.ProvenTxStatusUnmined),
			string(wdk.ProvenTxStatusCallback),
			string(wdk.ProvenTxStatusUnconfirmed),
			string(wdk.ProvenTxStatusCompleted),
			string(wdk.ProvenTxStatusReorg),
		}).
		Where("wasBroadcast = ?", false).
		UpdateColumn("wasBroadcast", true).
		Error
}
