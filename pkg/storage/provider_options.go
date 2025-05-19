package storage

import (
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/actions"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

// ProviderOption is function for additional setup of Provider itself.
type ProviderOption func(*providerOptions)

type providerOptions struct {
	gormDB     *gorm.DB
	funder     actions.Funder
	randomizer wdk.Randomizer
}

// WithGORM sets the GORM database for the provider.
func WithGORM(gormDB *gorm.DB) ProviderOption {
	return func(o *providerOptions) {
		o.gormDB = gormDB
	}
}

// WithRandomizer sets the randomizer for the provider.
func WithRandomizer(randomizer wdk.Randomizer) ProviderOption {
	return func(o *providerOptions) {
		o.randomizer = randomizer
	}
}

func toOptions(opts []ProviderOption) *providerOptions {
	options := &providerOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}
