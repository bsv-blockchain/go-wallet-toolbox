package repo

import (
	"context"
	"fmt"

	models2 "github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
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
		models2.Setting{},
		models2.User{},
		models2.OutputBasket{},
		models2.CertificateField{},
		models2.Certificate{},
		models2.UserUTXO{},
		models2.Transaction{},
		models2.Output{},
		models2.ProvenTxReq{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate settings: %w", err)
	}

	return nil
}
