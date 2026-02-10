package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"gorm.io/gorm"
)

// ProviderOption is function for additional setup of Provider itself.
type ProviderOption = func(*ProviderConfig)

// ProviderConfig contains configuration and dependencies required for initializing and running a storage provider instance.
type ProviderConfig struct {
	DBConfig defs.Database
	GormDB   *gorm.DB // NOTE: GormDB overrides DBConfig if both are provided. When set, DBConfig is ignored.

	Funder              funder.Funder
	Randomizer          wdk.Randomizer
	BeefVerifierFactory func() wdk.BeefVerifier
	Logger              *slog.Logger

	SynchronizeTxStatusesConfig defs.SynchronizeTxStatuses
	FailAbandonedConfig         defs.FailAbandoned

	FeeModel   defs.FeeModel
	Commission defs.Commission

	BackgroundBroadcasterContext context.Context
	BackgroundBroadcasterChannel chan<- wdk.CurrentTxStatus
}

// WithConfig returns a ProviderOption that sets the ProviderConfig to the supplied cfg value.
func WithConfig(cfg ProviderConfig) ProviderOption {
	return func(o *ProviderConfig) {
		*o = cfg
	}
}

// WithGORM sets the GORM database for the provider.
func WithGORM(gormDB *gorm.DB) ProviderOption {
	return func(o *ProviderConfig) {
		o.GormDB = gormDB
	}
}

// WithRandomizer sets the Randomizer for the provider.
func WithRandomizer(randomizer wdk.Randomizer) ProviderOption {
	return func(o *ProviderConfig) {
		o.Randomizer = randomizer
	}
}

// WithBeefVerifier sets a custom BeefVerifier implementation for use in the provider options.
func WithBeefVerifier(beefVerifier wdk.BeefVerifier) ProviderOption {
	return func(o *ProviderConfig) {
		o.BeefVerifierFactory = func() wdk.BeefVerifier {
			return beefVerifier
		}
	}
}

// WithFunder sets the Funder implementation in the provider options.
// Use to customize how funding logic is handled within the provider.
func WithFunder(funder funder.Funder) ProviderOption {
	return func(o *ProviderConfig) {
		o.Funder = funder
	}
}

// WithSynchronizeTxStatuses sets the configuration for synchronizing transaction statuses with specified retry attempts.
func WithSynchronizeTxStatuses(config defs.SynchronizeTxStatuses) ProviderOption {
	return func(o *ProviderConfig) {
		o.SynchronizeTxStatusesConfig = config
	}
}

// WithFailAbandoned sets the configuration to fail abandoned transactions after a specified minimum age in seconds.
func WithFailAbandoned(config defs.FailAbandoned) ProviderOption {
	return func(o *ProviderConfig) {
		o.FailAbandonedConfig = config
	}
}

// WithFeeModel sets the fee model option for the storage provider using the given FeeModel configuration.
func WithFeeModel(feeModel defs.FeeModel) ProviderOption {
	return func(o *ProviderConfig) {
		o.FeeModel = feeModel
	}
}

// WithLogger sets a custom logger for the storage provider using the provided slog.Logger instance.
func WithLogger(logger *slog.Logger) ProviderOption {
	return func(o *ProviderConfig) {
		o.Logger = logger
	}
}

// WithCommission returns a ProviderOption that sets the Commission configuration for the storage provider.
// It applies the provided defs.Commission value to the provider options.
// This is useful for enabling or customizing Commission parameters during provider setup.
func WithCommission(commission defs.Commission) ProviderOption {
	return func(o *ProviderConfig) {
		o.Commission = commission
	}
}

// WithBackgroundBroadcasterContext sets the context used by the background broadcaster in provider options.
// It allows external cancellation and propagation of deadlines to background broadcasting routines.
// Returns a ProviderOption that applies the provided context to ProviderConfig.
// Use this to integrate long-running broadcaster tasks with lifecycle management.
func WithBackgroundBroadcasterContext(ctx context.Context) ProviderOption {
	return func(o *ProviderConfig) {
		o.BackgroundBroadcasterContext = ctx
	}
}

// WithBackgroundBroadcasterChannel sets the notification channel for the background broadcaster in provider options.
// This channel is used to send transaction status updates when transactions are broadcasted in the background.
// This same channel which is passed to the monitor to receive broadcasted transaction updates should be used.
func WithBackgroundBroadcasterChannel(txBroadcastedChannel chan<- wdk.CurrentTxStatus) ProviderOption {
	return func(o *ProviderConfig) {
		o.BackgroundBroadcasterChannel = txBroadcastedChannel
	}
}

// WithDBConfig sets the database configuration for the storage provider using the given defs.Database configuration.
func WithDBConfig(dbConfig defs.Database) ProviderOption {
	return func(o *ProviderConfig) {
		o.DBConfig = dbConfig
	}
}

func defaultProviderOptions(chaintracker chaintracker.ChainTracker) ProviderConfig {
	return ProviderConfig{
		DBConfig:                     defs.DefaultDBConfig(),
		Randomizer:                   randomizer.New(),
		BeefVerifierFactory:          func() wdk.BeefVerifier { return NewDefaultBeefVerifier(chaintracker) },
		SynchronizeTxStatusesConfig:  defs.DefaultSynchronizeTxStatuses(),
		FailAbandonedConfig:          defs.DefaultFailAbandoned(),
		FeeModel:                     defs.DefaultFeeModel(),
		Commission:                   defs.DefaultCommission(),
		Logger:                       slog.Default(),
		BackgroundBroadcasterContext: context.Background(),
	}
}

func (p *ProviderConfig) verify() error {
	if err := p.FeeModel.Validate(); err != nil {
		return fmt.Errorf("provided fee model is invalid: %w", err)
	}
	if err := p.Commission.Validate(); err != nil {
		return fmt.Errorf("provided Commission is invalid: %w", err)
	}
	return nil
}

func (p *ProviderConfig) beefVerifier() wdk.BeefVerifier {
	return p.BeefVerifierFactory()
}
