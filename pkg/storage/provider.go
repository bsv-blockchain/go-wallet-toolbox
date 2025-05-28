package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/internal/validate"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/randomizer"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/storage/internal/actions"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

// ErrAuthorization is an error that indicates that the user is not authorized to perform the action.
var ErrAuthorization = fmt.Errorf("access is denied due to an authorization error")

// Provider is a storage provider.
type Provider struct {
	Chain    defs.BSVNetwork
	Database *database.Database

	settings *wdk.TableSettings
	repo     *repo.Repositories
	actions  *actions.Actions
}

var _ wdk.WalletStorageProvider = (*Provider)(nil)

// GORMProviderConfig is a configuration for GORM storage provider.
type GORMProviderConfig struct {
	DB                    defs.Database
	Chain                 defs.BSVNetwork
	FeeModel              defs.FeeModel
	Commission            defs.Commission
	Services              wdk.Services
	SynchronizeTxStatuses defs.SynchronizeTxStatuses
}

// NewGORMProvider creates a new storage provider with GORM repository.
func NewGORMProvider(logger *slog.Logger, config GORMProviderConfig, opts ...ProviderOption) (*Provider, error) {
	if err := config.FeeModel.Validate(); err != nil {
		return nil, fmt.Errorf("invalid fee model: %w", err)
	}

	options := toOptions(opts)

	db, err := configureDatabase(logger, config.DB, options)
	if err != nil {
		return nil, err
	}

	repos := db.CreateRepositories()

	var transactionFunder funder.Funder
	if options.funder != nil {
		transactionFunder = options.funder
	} else {
		transactionFunder = db.CreateFunder(config.FeeModel)
	}

	var random wdk.Randomizer
	if options.randomizer != nil {
		random = options.randomizer
	} else {
		random = randomizer.New()
	}

	if config.Services == nil {
		logger.Warn("services is not set, some actions may not work")
	}

	return &Provider{
		Chain:    config.Chain,
		Database: db,

		repo:    repos,
		actions: actions.New(logger, transactionFunder, config.Commission, repos, random, config.Services, config.SynchronizeTxStatuses),
	}, nil
}

func configureDatabase(logger *slog.Logger, dbConfig defs.Database, options *providerOptions) (*database.Database, error) {
	if options.gormDB != nil {
		return database.NewWithGorm(options.gormDB, logger), nil
	}

	db, err := database.NewDatabase(dbConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	return db, nil
}

// Migrate migrates the storage and saves the settings.
func (p *Provider) Migrate(ctx context.Context, storageName string, storageIdentityKey string) (string, error) {
	err := p.repo.Migrate(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to migrate: %w", err)
	}

	// TODO: what if p.Chain != Chain from DB?

	err = p.repo.SaveSettings(ctx, &wdk.TableSettings{
		StorageIdentityKey: storageIdentityKey,
		StorageName:        storageName,
		Chain:              p.Chain,
		MaxOutputScript:    DefaultMaxScriptLength,
	})
	if err != nil {
		return "", fmt.Errorf("failed to save settings: %w", err)
	}

	// NOTE: GORM automigrate does not support db versioning
	// from-kt: In TS version I can't find any usage of returned version
	version := "auto-migrated"

	return version, nil
}

// MakeAvailable reads the settings and makes them available.
func (p *Provider) MakeAvailable(ctx context.Context) (*wdk.TableSettings, error) {
	settings, err := p.repo.ReadSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}

	p.settings = settings
	return settings, nil
}

// InsertCertificateAuth inserts certificate to the database for authenticated user
func (p *Provider) InsertCertificateAuth(ctx context.Context, auth wdk.AuthID, certificate *wdk.TableCertificateX) (uint, error) {
	if auth.UserID == nil || certificate.UserID != *auth.UserID {
		return 0, ErrAuthorization
	}

	err := validate.TableCertificateX(certificate)
	if err != nil {
		return 0, fmt.Errorf("invalid insertCertificateAuth args: %w", err)
	}

	certModel := &models.Certificate{
		Type:               string(certificate.Type),
		SerialNumber:       string(certificate.SerialNumber),
		Certifier:          string(certificate.Certifier),
		Subject:            string(certificate.Subject),
		RevocationOutpoint: string(certificate.RevocationOutpoint),
		Signature:          string(certificate.Signature),

		UserID:            *auth.UserID,
		CertificateFields: slices.Map(certificate.Fields, tableCertificateXFieldsToModelFields(*auth.UserID)),
	}

	if certificate.Verifier != nil {
		certModel.Verifier = string(*certificate.Verifier)
	}

	id, err := p.repo.CreateCertificate(ctx, certModel)
	if err != nil {
		return 0, fmt.Errorf("failed to create certificate: %w", err)
	}

	return id, nil
}

// RelinquishCertificate will relinquish existing certificate
func (p *Provider) RelinquishCertificate(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishCertificateArgs) error {
	if auth.UserID == nil {
		return ErrAuthorization
	}

	err := validate.RelinquishCertificateArgs(&args)
	if err != nil {
		return fmt.Errorf("invalid relinquishCertificate args: %w", err)
	}

	err = p.repo.DeleteCertificate(ctx, *auth.UserID, args)
	if err != nil {
		return fmt.Errorf("failed to relinquish certificate: %w", err)
	}

	return nil
}

// ListCertificates will list certificates with provided args
func (p *Provider) ListCertificates(ctx context.Context, auth wdk.AuthID, args wdk.ListCertificatesArgs) (*wdk.ListCertificatesResult, error) {
	if auth.UserID == nil {
		return nil, ErrAuthorization
	}

	err := validate.ListCertificatesArgs(&args)
	if err != nil {
		return nil, fmt.Errorf("invalid listCertificates args: %w", err)
	}

	// prepare arguments
	filterOptions := listCertificatesArgsToActionParams(args)

	certModels, totalCount, err := p.repo.ListAndCountCertificates(ctx, *auth.UserID, filterOptions)
	if err != nil {
		return nil, fmt.Errorf("error during listing certificates action: %w", err)
	}

	tc, err := to.UInt(totalCount)
	if err != nil {
		return nil, fmt.Errorf("error during parsing total count of certificates: %w", err)
	}

	result := &wdk.ListCertificatesResult{
		TotalCertificates: primitives.PositiveInteger(tc),
		Certificates:      slices.Map(certModels, certModelToResult),
	}

	return result, nil
}

// FindOrInsertUser will find user by their identityKey or inserts a new one if not found
func (p *Provider) FindOrInsertUser(ctx context.Context, identityKey string) (*wdk.FindOrInsertUserResponse, error) {
	user, err := p.repo.FindUser(ctx, identityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	if user != nil {
		return &wdk.FindOrInsertUserResponse{
			User:  *user,
			IsNew: false,
		}, nil
	}

	settings, err := p.repo.ReadSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}

	user, err = p.repo.CreateUser(
		ctx,
		identityKey,
		settings.StorageIdentityKey,
		wdk.DefaultBasketConfiguration(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	return &wdk.FindOrInsertUserResponse{
		User:  *user,
		IsNew: true,
	}, nil
}

// CreateAction Storage level processing for wallet `createAction`.
func (p *Provider) CreateAction(ctx context.Context, auth wdk.AuthID, args wdk.ValidCreateActionArgs) (*wdk.StorageCreateActionResult, error) {
	if auth.UserID == nil {
		return nil, ErrAuthorization
	}
	if err := validate.ValidCreateActionArgs(&args); err != nil {
		return nil, fmt.Errorf("invalid createAction args: %w", err)
	}

	res, err := p.actions.Create(ctx, *auth.UserID, actions.FromValidCreateActionArgs(&args))
	if err != nil {
		return nil, fmt.Errorf("failed to process createAction: %w", err)
	}
	return res, nil
}

// InternalizeAction Storage level processing for wallet `internalizeAction`.
func (p *Provider) InternalizeAction(ctx context.Context, auth wdk.AuthID, args wdk.InternalizeActionArgs) (*wdk.InternalizeActionResult, error) {
	if auth.UserID == nil {
		return nil, ErrAuthorization
	}
	if err := validate.ValidInternalizeActionArgs(&args); err != nil {
		return nil, fmt.Errorf("invalid internalizeAction args: %w", err)
	}

	res, err := p.actions.Internalize(ctx, *auth.UserID, &args)
	if err != nil {
		return nil, fmt.Errorf("failed to process internalizeAction: %w", err)
	}
	return res, nil
}

// ProcessAction Storage level processing for wallet `processAction`.
func (p *Provider) ProcessAction(ctx context.Context, auth wdk.AuthID, args wdk.ProcessActionArgs) (*wdk.ProcessActionResult, error) {
	if auth.UserID == nil {
		return nil, ErrAuthorization
	}
	if err := validate.ProcessActionArgs(&args); err != nil {
		return nil, fmt.Errorf("invalid processAction args: %w", err)
	}

	res, err := p.actions.Process(ctx, *auth.UserID, &args)
	if err != nil {
		return nil, fmt.Errorf("failed to process processAction: %w", err)
	}
	return res, nil
}

// SynchronizeTransactionStatuses synchronizes the statuses of tracked transactions with the current network state.
func (p *Provider) SynchronizeTransactionStatuses(ctx context.Context) error {
	err := p.actions.SynchronizeTxStatuses(ctx)
	if err != nil {
		return fmt.Errorf("failed to synchronize transaction statuses: %w", err)
	}
	return nil
}

// ListOutputs will list outputs with provided args
func (p *Provider) ListOutputs(ctx context.Context, auth wdk.AuthID, args wdk.ListOutputsArgs) (*wdk.ListOutputsResult, error) {
	if auth.UserID == nil {
		return nil, ErrAuthorization
	}

	if err := validate.ListOutputsArgs(&args); err != nil {
		return nil, fmt.Errorf("invalid listOutputs args: %w", err)
	}

	result, err := p.actions.ListOutputs(ctx, auth, &args)
	if err != nil {
		return nil, fmt.Errorf("failed to list outputs: %w", err)
	}
	return result, nil
}
