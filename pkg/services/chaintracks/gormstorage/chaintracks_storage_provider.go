package gormstorage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services/chaintracks/models"
	"github.com/go-softwarelab/common/pkg/to"
)

// Provider manages access to the database and repositories, coordinating data storage, retrieval, and migration operations.
type Provider struct {
	Database *database.Database

	logger *slog.Logger
}

// NewProvider creates and returns a new Provider instance using the given logger and optional configuration options.
// Returns an error if database configuration or repository initialization fails.
func NewProvider(logger *slog.Logger, opts ...ProviderOption) (*Provider, error) {
	logger = logging.Child(logger, "chaintracks_gorm_storage_provider")
	options := to.OptionsWithDefault(defaultProviderOptions(), opts...)

	db, err := configureDatabase(logger, options.DBConfig, &options)
	if err != nil {
		return nil, err
	}

	return &Provider{
		Database: db,

		logger: logger,
	}, nil
}

// Migrate migrates the storage and saves the settings.
func (p *Provider) Migrate(ctx context.Context) error {
	migrator := repo.NewMigrator(p.Database.DB)
	err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	return nil
}

func configureDatabase(logger *slog.Logger, dbConfig defs.Database, options *ProviderConfig) (*database.Database, error) {
	if options.GormDB != nil {
		return database.NewWithGorm(options.GormDB, logger), nil
	}

	db, err := database.NewDatabase(dbConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	return db, nil
}

// Query returns a StorageQueries interface for live blockchain header operations using the provider's database context.
func (p *Provider) Query(ctx context.Context) models.StorageQueries {
	return newStorageQueries(ctx, p.Database.DB)
}
