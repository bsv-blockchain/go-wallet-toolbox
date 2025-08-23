package storage

import (
	"fmt"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

// ProviderOption is function for additional setup of Provider itself.
type ProviderOption = func(*providerOptions)

type providerOptions struct {
	gormDB       *gorm.DB
	funder       funder.Funder
	randomizer   wdk.Randomizer
	beefVerifier wdk.BeefVerifier

	synchronizeTxStatusesConfig defs.SynchronizeTxStatuses
	failAbandonedConfig         defs.FailAbandoned

	feeModel   defs.FeeModel
	commission defs.Commission
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

// WithFunder sets the funder implementation in the provider options.
// Use to customize how funding logic is handled within the provider.
func WithFunder(funder funder.Funder) ProviderOption {
	return func(o *providerOptions) {
		o.funder = funder
	}
}

// WithSynchronizeTxStatuses sets the configuration for synchronizing transaction statuses with specified retry attempts.
func WithSynchronizeTxStatuses(config defs.SynchronizeTxStatuses) ProviderOption {
	return func(o *providerOptions) {
		o.synchronizeTxStatusesConfig = config
	}
}

// WithFailAbandoned sets the configuration to fail abandoned transactions after a specified minimum age in seconds.
func WithFailAbandoned(config defs.FailAbandoned) ProviderOption {
	return func(o *providerOptions) {
		o.failAbandonedConfig = config
	}
}

// WithFeeModel sets the fee model option for the storage provider using the given FeeModel configuration.
func WithFeeModel(feeModel defs.FeeModel) ProviderOption {
	return func(o *providerOptions) {
		o.feeModel = feeModel
	}
}

// WithCommission returns a ProviderOption that sets the commission configuration for the storage provider.
// It applies the provided defs.Commission value to the provider options.
// This is useful for enabling or customizing commission parameters during provider setup.
func WithCommission(commission defs.Commission) ProviderOption {
	return func(o *providerOptions) {
		o.commission = commission
	}
}

func defaultProviderOptions() providerOptions {
	return providerOptions{
		randomizer:                  randomizer.New(),
		beefVerifier:                &DefaultBeefVerifier{},
		synchronizeTxStatusesConfig: defs.DefaultSynchronizeTxStatuses(),
		failAbandonedConfig:         defs.DefaultFailAbandoned(),
		feeModel:                    defs.DefaultFeeModel(),
		commission:                  defs.DefaultCommission(),
	}
}

func (p *providerOptions) verify() error {
	if err := p.feeModel.Validate(); err != nil {
		return fmt.Errorf("provided fee model is invalid: %w", err)
	}
	if err := p.commission.Validate(); err != nil {
		return fmt.Errorf("provided commission is invalid: %w", err)
	}
	return nil
}
