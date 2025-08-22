package storage

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

// ProviderOption is function for additional setup of Provider itself.
type ProviderOption func(*providerOptions)

type providerOptions struct {
	gormDB       *gorm.DB
	funder       funder.Funder
	randomizer   wdk.Randomizer
	beefVerifier wdk.BeefVerifier
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

// WithBeefVerifier sets a custom BeefVerifier implementation for use in the provider options.
func WithBeefVerifier(beefVerifier wdk.BeefVerifier) ProviderOption {
	return func(o *providerOptions) {
		o.beefVerifier = beefVerifier
	}
}

func toOptions(opts []ProviderOption) *providerOptions {
	options := &providerOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}
