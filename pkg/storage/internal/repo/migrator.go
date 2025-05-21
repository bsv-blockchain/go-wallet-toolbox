package repo

import (
	"context"
	"fmt"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/database/models"
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
		models.ProvenTxReq{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate settings: %w", err)
	}

	return nil
}
