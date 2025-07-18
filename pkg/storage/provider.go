package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/database/models"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/entity"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/funder"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/storage/repo"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/crud"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/actions"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/sync"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/slices"
	"github.com/go-softwarelab/common/pkg/to"
)

// ErrAuthorization is an error that indicates that the user is not authorized to perform the action.
var ErrAuthorization = fmt.Errorf("access is denied due to an authorization error")

// Provider is a storage provider.
type Provider struct {
	Chain    defs.BSVNetwork
	Database *database.Database

	repo         *repo.Repositories
	actions      *actions.Actions
	random       wdk.Randomizer
	parentLogger *slog.Logger
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

		repo:         repos,
		actions:      actions.New(logger, transactionFunder, config.Commission, repos, random, config.Services, config.SynchronizeTxStatuses),
		random:       random,
		parentLogger: logger,
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
			User:  *user.ToWDK(),
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
		User:  *user.ToWDK(),
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

	p.parentLogger.Info("Starting CreateAction process",
		slog.Int("userID", *auth.UserID),
		slog.String("description", string(args.Description)),
		slog.Int("outputCount", len(args.Outputs)),
		slog.Int("inputCount", len(args.Inputs)),
		slog.Bool("isSignAction", args.IsSignAction),
	)

	res, err := p.actions.Create(ctx, *auth.UserID, actions.FromValidCreateActionArgs(&args))
	if err != nil {
		return nil, fmt.Errorf("failed to process createAction: %w", err)
	}

	p.parentLogger.Info("CreateAction completed successfully",
		slog.Int("userID", *auth.UserID),
		slog.String("reference", res.Reference),
		slog.Int("resultOutputCount", len(res.Outputs)),
		slog.Int("resultInputCount", len(res.Inputs)),
	)

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

// RelinquishOutput removes a specified output from a basket
func (p *Provider) RelinquishOutput(ctx context.Context, auth wdk.AuthID, args wdk.RelinquishOutputArgs) error {
	if auth.UserID == nil {
		return ErrAuthorization
	}

	if err := validate.ValidRelinquishOutputArgs(&args); err != nil {
		return fmt.Errorf("invalid relinquishOutput args: %w", err)
	}

	txID, vout := primitives.OutpointString(args.Output).MustGet()

	var basketName *string
	if args.Basket != "" {
		basketName = &args.Basket
	}

	err := p.repo.Outputs.UnlinkOutputFromBasketByOutpoint(ctx, *auth.UserID, basketName, wdk.OutPoint{TxID: txID, Vout: vout})
	if err != nil {
		return fmt.Errorf("failed to relinquish output: %w", err)
	}
	return nil
}

// ConfigureBasket validates and updates the basket configuration for the authorized user in the repository.
// Returns an error if the user is unauthorized, input is invalid, or the update fails.
// NOTE: For "change basket" use wdk.BasketNameForChange ("default") as the basket name.
func (p *Provider) ConfigureBasket(ctx context.Context, auth wdk.AuthID, args wdk.BasketConfiguration) error {
	if auth.UserID == nil {
		return ErrAuthorization
	}

	if err := validate.ValidBasketConfiguration(&args); err != nil {
		return fmt.Errorf("invalid basket configuration: %w", err)
	}

	_, err := p.repo.UpsertOutputBasket(ctx, *auth.UserID, args)
	if err != nil {
		return fmt.Errorf("failed to update basket configuration: %w", err)
	}
	return nil
}

// ListActions will list actions with provided args
// It returns a paginated list of actions for the authenticated user.
// The result includes the total number of actions and the actions themselves.
func (p *Provider) ListActions(ctx context.Context, auth wdk.AuthID, args wdk.ListActionsArgs) (*wdk.ListActionsResult, error) {
	if auth.UserID == nil {
		return nil, ErrAuthorization
	}

	if err := validate.ListActionsArgs(&args); err != nil {
		return nil, fmt.Errorf("invalid listActions args: %w", err)
	}

	result, err := p.actions.ListActions(ctx, auth, &args)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}
	return result, nil
}

// GetSyncChunk retrieves a sync chunk based on the provided arguments.
// It returns the requested sync chunk or an error if retrieval fails.
func (p *Provider) GetSyncChunk(ctx context.Context, args wdk.RequestSyncChunkArgs) (*wdk.SyncChunk, error) {
	settings, err := p.repo.ReadSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}

	if settings.StorageIdentityKey != args.FromStorageIdentityKey {
		return nil, fmt.Errorf("fromStorageIdentityKey %s does not match the storage identity key %s", args.FromStorageIdentityKey, settings.StorageIdentityKey)
	}

	if err := validate.ValidRequestSyncChunkArgs(&args); err != nil {
		return nil, fmt.Errorf("invalid requestSyncChunk args: %w", err)
	}

	chunk, err := sync.NewGetSyncChunkAction(p.parentLogger, p.repo, &args).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync chunk: %w", err)
	}
	return chunk, nil
}

// FindOrInsertSyncStateAuth finds or inserts a sync state for the given user, storage identity key, and storage name.
func (p *Provider) FindOrInsertSyncStateAuth(ctx context.Context, auth wdk.AuthID, storageIdentityKey, storageName string) (*wdk.FindOrInsertSyncStateAuthResponse, error) {
	if auth.UserID == nil {
		return nil, ErrAuthorization
	}

	action := sync.NewFindOrInsertSyncState(p.repo, p.random, *auth.UserID, storageIdentityKey, storageName)
	syncStateResponse, err := action.FindOrInsertSyncState(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find or insert sync state: %w", err)
	}

	return syncStateResponse, nil
}

// ProcessSyncChunk validates arguments and processes a synchronization chunk, returning the processing result or an error.
func (p *Provider) ProcessSyncChunk(ctx context.Context, args wdk.RequestSyncChunkArgs, chunk *wdk.SyncChunk) (*wdk.ProcessSyncChunkResult, error) {
	err := validate.ValidRequestSyncChunkArgs(&args)
	if err != nil {
		return nil, fmt.Errorf("invalid requestSyncChunk args: %w", err)
	}

	user, err := p.repo.FindUser(ctx, args.IdentityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user with identity key %s not found", args.IdentityKey)
	}

	result, err := sync.NewChunkProcessor(ctx, p.repo, chunk, &args, user).Process()
	if err != nil {
		return nil, fmt.Errorf("failed to process chunk: %w", err)
	}

	return result, nil
}

// FindUserTransactionByReference retrieves a user transaction by userID and its reference.
// NOTE: It returns nil if the transaction is not found.
func (p *Provider) FindUserTransactionByReference(ctx context.Context, userID int, reference string) (*entity.Transaction, error) {
	txEntity, err := p.repo.Transactions.FindTransactionByReference(ctx, userID, reference)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction by ID: %w", err)
	}

	return txEntity, nil
}

// CommissionEntity returns a Commission interface for querying and filtering commission records in the storage provider.
func (p *Provider) CommissionEntity() crud.Commission {
	return crud.NewCommission(p.repo.Commission)
}

// KnownTxEntity returns an accessor to perform read operations on known transactions in the underlying repository.
func (p *Provider) KnownTxEntity() crud.KnownTx {
	return crud.NewKnownTx(p.repo.KnownTx)
}
