package infra

import "github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"

// GORMProviderOptionsFromConfig returns a slice of ProviderOption based on the given Config values for GORM storage setup.
func GORMProviderOptionsFromConfig(cfg *Config) []storage.ProviderOption {
	return []storage.ProviderOption{
		storage.WithDBConfig(cfg.DBConfig),
		storage.WithFeeModel(cfg.FeeModel),
		storage.WithCommission(cfg.Commission),
		storage.WithSynchronizeTxStatuses(cfg.SynchronizeTxStatuses),
		storage.WithFailAbandoned(cfg.FailAbandoned),
		storage.WithChangeBasket(cfg.ChangeBasket),
	}
}
