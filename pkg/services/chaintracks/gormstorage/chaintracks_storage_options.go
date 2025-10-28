package gormstorage

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"gorm.io/gorm"
)

// ProviderOption is function for additional setup of Provider itself.
type ProviderOption = func(*ProviderConfig)

// ProviderConfig contains configuration and dependencies required for initializing and running a storage provider instance.
type ProviderConfig struct {
	DBConfig defs.Database
	GormDB   *gorm.DB // NOTE: GormDB overrides DBConfig if both are provided. When set, DBConfig is ignored.
}

func defaultProviderOptions() ProviderConfig {
	return ProviderConfig{
		DBConfig: defs.DefaultDBConfig(),
	}
}

// WithDBConfig sets the database configuration for the storage provider using the given defs.Database configuration.
func WithDBConfig(dbConfig defs.Database) ProviderOption {
	return func(o *ProviderConfig) {
		o.DBConfig = dbConfig
	}
}

// WithGORM sets the GORM database for the provider.
func WithGORM(gormDB *gorm.DB) ProviderOption {
	return func(o *ProviderConfig) {
		o.GormDB = gormDB
	}
}
