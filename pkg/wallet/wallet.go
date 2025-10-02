package wallet

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	sdk "github.com/bsv-blockchain/go-sdk/wallet"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/logging"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/specops"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/validate"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/services"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/actions"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/mapping"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/internal/wallet_opts"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet/pending"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/go-softwarelab/common/pkg/slogx"
	"github.com/go-softwarelab/common/pkg/to"
)

var _ sdk.Interface = (*Wallet)(nil)

type walletCleanupFunc func()

func (wc walletCleanupFunc) Add(next func()) walletCleanupFunc {
	if wc == nil {
		return next
	}
	return func() {
		wc()
		next()
	}
}

// Wallet is an implementation of the BRC-100 wallet interface.
type Wallet struct {
	proto                   *sdk.ProtoWallet
	storage                 wdk.WalletStorage
	keyDeriver              *sdk.KeyDeriver
	flags                   *wallet_opts.Flags
	services                *services.WalletServices
	chain                   defs.BSVNetwork
	pendingSignActionsCache pending.SignActionsRepository
	logger                  *slog.Logger
	cleanup                 walletCleanupFunc
}

// WithIncludeAllSourceTransactions - default: `true`
// If true, signableTransactions will include sourceTransaction for each input,
// including those that do not require signature and those that were also contained in the inputBEEF.
func WithIncludeAllSourceTransactions(value bool) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.IncludeAllSourceTransactions = value
	}
}

// WithAutoKnownTxids - default: `false`
// If true, txids that are known to the wallet's party beef do not need to be returned from storage.
func WithAutoKnownTxids(value bool) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.AutoKnownTxids = value
	}
}

// WithTrustSelf - default: `known`
// controls behavior of input BEEF validation.
// If "known", input transactions may omit supporting validity proof data for all TXIDs known to this wallet.
// If "", input BEEFs must be complete and valid.
func WithTrustSelf(value sdk.TrustSelf) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		if value == "" {
			opts.TrustSelf = nil
		} else {
			opts.TrustSelf = &value
		}
	}
}

// WithServices allows to set the wallet services that will be used by the wallet.
func WithServices(services *services.WalletServices) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.Services = services
	}
}

// WithPendingSignActionsRepository sets the SignActionsRepository for wallet options, allowing management of cached actions.
func WithPendingSignActionsRepository(cache pending.SignActionsRepository) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		opts.PendingSignActionsRepo = cache
	}
}

// WithLogger sets the provided slog.Logger to the Logger field in wallet_opts.Opts if the logger is not nil.
func WithLogger(logger *slog.Logger) func(*wallet_opts.Opts) {
	return func(opts *wallet_opts.Opts) {
		if logger != nil {
			opts.Logger = logger
		}
	}
}

// New creates a new Wallet instance with the specified network, key deriver, and storage.
// Returns an error if any required parameter is invalid or missing.
func New[KeySource PrivateKeySource](chain defs.BSVNetwork, keySource KeySource, activeStorage wdk.WalletStorageProvider, opts ...func(*wallet_opts.Opts)) (*Wallet, error) {
	if activeStorage == nil {
		return nil, fmt.Errorf("active storage must be provided")
	}

	return NewWithStorageFactory(chain, keySource, func() wdk.WalletStorageProvider { return activeStorage }, opts...)
}

// NewWithStorageFactory creates a new Wallet instance with the specified network, key deriver, and storage created with provided storage factory function
func NewWithStorageFactory[KeySource PrivateKeySource, ActiveStorageFactory StorageProviderFactory](chain defs.BSVNetwork, keySource KeySource, activeStorageFactory ActiveStorageFactory, opts ...func(*wallet_opts.Opts)) (*Wallet, error) {
	err := chain.Validate()
	if err != nil {
		return nil, fmt.Errorf("valid chain must be provided: %w", err)
	}

	if activeStorageFactory == nil {
		return nil, fmt.Errorf("active storage factory must be provided")
	}

	options := to.OptionsWithDefault(wallet_opts.Opts{
		Flags: wallet_opts.Flags{
			IncludeAllSourceTransactions: true,
			AutoKnownTxids:               false,
			TrustSelf:                    to.Ptr(sdk.TrustSelfKnown),
		},
		Logger:                 slog.Default(),
		Services:               nil,
		PendingSignActionsRepo: nil,
	}, opts...)

	logger := logging.Child(options.Logger, "wallet")

	if options.PendingSignActionsRepo == nil {
		options.PendingSignActionsRepo = pending.NewSignActionLocalRepository(logger, pending.DefaultPendingSignActionsTTL)
	}

	keyDeriver, err := toKeyDeriver(keySource)
	if err != nil {
		return nil, fmt.Errorf("failed to create key deriver from key source: %w", err)
	}

	proto, err := sdk.NewProtoWallet(sdk.ProtoWalletArgs{Type: sdk.ProtoWalletArgsTypeKeyDeriver, KeyDeriver: keyDeriver})
	if err != nil {
		return nil, fmt.Errorf("failed to create proto wallet: %w", err)
	}

	w := &Wallet{
		proto:                   proto,
		keyDeriver:              keyDeriver,
		flags:                   &options.Flags,
		services:                options.Services,
		chain:                   chain,
		pendingSignActionsCache: options.PendingSignActionsRepo,
		logger:                  logger,
	}

	activeStorage, storageCleanup, err := toStorageProvider(w, activeStorageFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create active storage: %w", err)
	}
	w.cleanup = w.cleanup.Add(storageCleanup)

	storageManager := storage.NewWalletStorageManager(keyDeriver.IdentityKey().ToDERHex(), logger, activeStorage)
	w.storage = storageManager

	return w, nil
}

// GetPublicKey retrieves a derived or identity public key based on the requested protocol, key ID, counterparty, and other factors.
func (w *Wallet) GetPublicKey(ctx context.Context, args sdk.GetPublicKeyArgs, originator string) (*sdk.GetPublicKeyResult, error) {
	logging.Child(w.logger, "GetPublicKey").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.GetPublicKey(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}
	return res, nil
}

// Encrypt encrypts the provided plaintext data using derived keys, based on the protocol ID, key ID, counterparty, and other factors.
func (w *Wallet) Encrypt(ctx context.Context, args sdk.EncryptArgs, originator string) (*sdk.EncryptResult, error) {
	logging.Child(w.logger, "Encrypt").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.Encrypt(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}
	return res, nil
}

// Decrypt decrypts the provided ciphertext using derived keys, based on the protocol ID, key ID, counterparty, and other factors.
func (w *Wallet) Decrypt(ctx context.Context, args sdk.DecryptArgs, originator string) (*sdk.DecryptResult, error) {
	logging.Child(w.logger, "Decrypt").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.Decrypt(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return res, nil
}

// CreateHMAC creates an HMAC (Hash-based Message Authentication Code) based on the provided data, protocol, key ID, counterparty, and other factors.
func (w *Wallet) CreateHMAC(ctx context.Context, args sdk.CreateHMACArgs, originator string) (*sdk.CreateHMACResult, error) {
	logging.Child(w.logger, "CreateHMAC").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.CreateHMAC(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to create HMAC: %w", err)
	}
	return res, nil
}

// VerifyHMAC verifies an HMAC (Hash-based Message Authentication Code) based on the provided data, protocol, key ID, counterparty, and other factors.
func (w *Wallet) VerifyHMAC(ctx context.Context, args sdk.VerifyHMACArgs, originator string) (*sdk.VerifyHMACResult, error) {
	logging.Child(w.logger, "VerifyHMAC").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.VerifyHMAC(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to verify HMAC: %w", err)
	}
	return res, nil
}

// CreateSignature creates a digital signature for the provided data or hash using a specific protocol, key, and optionally considering privilege and counterparty.
func (w *Wallet) CreateSignature(ctx context.Context, args sdk.CreateSignatureArgs, originator string) (*sdk.CreateSignatureResult, error) {
	logging.Child(w.logger, "CreateSignature").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.CreateSignature(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to create signature: %w", err)
	}
	return res, nil
}

// VerifySignature verifies a digital signature for the provided data or hash using a specific protocol, key, and optionally considering privilege and counterparty.
func (w *Wallet) VerifySignature(ctx context.Context, args sdk.VerifySignatureArgs, originator string) (*sdk.VerifySignatureResult, error) {
	logging.Child(w.logger, "VerifySignature").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.VerifySignature(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}
	return res, nil
}

// CreateAction creates a new Bitcoin transaction based on the provided inputs, outputs, labels, locks, and other options.
func (w *Wallet) CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error) {
	logger := logging.Child(w.logger, "CreateAction")
	logger.DebugContext(ctx, "start", slogx.String("originator", originator))
	start := time.Now()
	defer func() { logger.DebugContext(ctx, "done", slog.Duration("duration", time.Since(start))) }()
	action := &actions.CreateAction{
		KeyDeriver:              w.keyDeriver,
		Storage:                 w.storage,
		WalletOpts:              w.flags,
		PendingSignActionsCache: w.pendingSignActionsCache,
	}

	result, err := action.CreateAction(ctx, args, originator)
	if err != nil {
		logger.DebugContext(ctx, "error", slogx.Error(err))
		return nil, fmt.Errorf("create action failed: %w", err)
	}
	logger.DebugContext(ctx, "success")
	return result, nil
}

// SignAction signs a transaction previously created using CreateAction.
func (w *Wallet) SignAction(ctx context.Context, args sdk.SignActionArgs, originator string) (*sdk.SignActionResult, error) {
	logger := logging.Child(w.logger, "SignAction")
	logger.DebugContext(ctx, "start", slogx.String("originator", originator))
	start := time.Now()
	defer func() { logger.DebugContext(ctx, "done", slog.Duration("duration", time.Since(start))) }()
	action := &actions.SignAction{
		Logger:                  w.logger,
		PendingSignActionsCache: w.pendingSignActionsCache,
		Storage:                 w.storage,
	}

	result, err := action.SignAction(ctx, args, originator)
	if err != nil {
		logger.DebugContext(ctx, "error", slogx.Error(err))
		return nil, fmt.Errorf("sign action failed: %w", err)
	}
	logger.DebugContext(ctx, "success")
	return result, nil
}

// AbortAction aborts a transaction that is in progress and has not yet been finalized or sent to the network.
func (w *Wallet) AbortAction(ctx context.Context, args sdk.AbortActionArgs, originator string) (*sdk.AbortActionResult, error) {
	logging.Child(w.logger, "AbortAction").DebugContext(ctx, "call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapAbortActionArgs(args)

	if err := validate.ValidAbortActionArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid abort action args: %w", err)
	}

	result, err := w.storage.AbortAction(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to abort action: %w", err)
	}

	return mapping.MapAbortActionResult(result), nil
}

// ListActions lists all transactions matching the specified labels.
func (w *Wallet) ListActions(ctx context.Context, args sdk.ListActionsArgs, originator string) (*sdk.ListActionsResult, error) {
	logging.Child(w.logger, "ListActions").DebugContext(ctx, "call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapListActionsArgs(args)

	if err := validate.ListActionsArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid list actions args: %w", err)
	}

	result, err := w.storage.ListActions(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}

	mappedResult, err := mapping.MapListActionsResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to map list actions result: %w", err)
	}

	return mappedResult, nil
}

// ListFailedActions returns only actions with status 'failed'. If unfail is true, it also requests recovery by adding the 'unfail' label.
func (w *Wallet) ListFailedActions(ctx context.Context, args sdk.ListActionsArgs, unfail bool, originator string) (*sdk.ListActionsResult, error) {
	logging.Child(w.logger, "ListFailedActions").DebugContext(ctx, "call", slogx.String("originator", originator), slog.Bool("unfail", unfail))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	args.Labels = append(args.Labels, specops.ListActionsSpecOpFailedActionsLabel)
	if unfail {
		args.Labels = append(args.Labels, "unfail")
	}

	wdkArgs := mapping.MapListActionsArgs(args)

	if err := validate.ListActionsArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid list actions args: %w", err)
	}

	result, err := w.storage.ListActions(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}

	mappedResult, err := mapping.MapListActionsResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to map list actions result: %w", err)
	}

	return mappedResult, nil
}

// InternalizeAction submits a transaction to be internalized and optionally labeled, outputs paid to the wallet balance,
// inserted into baskets, and/or tagged.
func (w *Wallet) InternalizeAction(ctx context.Context, args sdk.InternalizeActionArgs, originator string) (*sdk.InternalizeActionResult, error) {
	logging.Child(w.logger, "InternalizeAction").DebugContext(ctx, "call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapInternalizeActionArgs(args)

	if err := validate.WalletInternalizeAction(w.keyDeriver, &wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid internalize action args: %w", err)
	}

	result, err := w.storage.InternalizeAction(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to internalize action: %w", err)
	}

	return mapping.MapInternalizeActionResult(result), nil
}

// ListOutputs lists the spendable outputs kept within a specific basket, optionally tagged with specific labels.
func (w *Wallet) ListOutputs(ctx context.Context, args sdk.ListOutputsArgs, originator string) (*sdk.ListOutputsResult, error) {
	logging.Child(w.logger, "ListOutputs").DebugContext(ctx, "call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapListOutputsArgs(args)

	if err := validate.ListOutputsArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid list outputs args: %w", err)
	}

	result, err := w.storage.ListOutputs(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to list outputs: %w", err)
	}

	mappedResult, err := mapping.MapListOutputsResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to map list outputs result: %w", err)
	}

	return mappedResult, nil
}

// RelinquishOutput relinquishes an output from a basket, removing it from tracking without spending it.
func (w *Wallet) RelinquishOutput(ctx context.Context, args sdk.RelinquishOutputArgs, originator string) (*sdk.RelinquishOutputResult, error) {
	logging.Child(w.logger, "RelinquishOutput").DebugContext(ctx, "call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkArgs := mapping.MapRelinquishOutputArgs(args)

	if err := validate.ValidRelinquishOutputArgs(&wdkArgs); err != nil {
		return nil, fmt.Errorf("invalid relinquish output args: %w", err)
	}

	err := w.storage.RelinquishOutput(ctx, wdkArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to relinquish output: %w", err)
	}

	return &sdk.RelinquishOutputResult{
		Relinquished: true,
	}, nil
}

// RevealCounterpartyKeyLinkage reveals the key linkage between ourselves and a counterparty, to a particular verifier,
// across all interactions with the counterparty.
func (w *Wallet) RevealCounterpartyKeyLinkage(ctx context.Context, args sdk.RevealCounterpartyKeyLinkageArgs, originator string) (*sdk.RevealCounterpartyKeyLinkageResult, error) {
	logging.Child(w.logger, "RevealCounterpartyKeyLinkage").DebugContext(ctx, "call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.RevealCounterpartyKeyLinkage(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to reveal counterparty key linkage: %w", err)
	}
	return res, nil
}

// RevealSpecificKeyLinkage reveals the key linkage between ourselves and a counterparty, to a particular verifier,
// with respect to a specific interaction.
func (w *Wallet) RevealSpecificKeyLinkage(ctx context.Context, args sdk.RevealSpecificKeyLinkageArgs, originator string) (*sdk.RevealSpecificKeyLinkageResult, error) {
	logging.Child(w.logger, "RevealSpecificKeyLinkage").DebugContext(ctx, "call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	// TODO: support for privileged key manager (https://github.com/bitcoin-sv/wallet-toolbox/blob/master/src/sdk/PrivilegedKeyManager.ts)
	res, err := w.proto.RevealSpecificKeyLinkage(ctx, args, originator)
	if err != nil {
		return nil, fmt.Errorf("failed to reveal specific key linkage: %w", err)
	}
	return res, nil
}

// AcquireCertificate acquires an identity certificate, whether by acquiring one from the certifier or by directly receiving it.
func (w *Wallet) AcquireCertificate(ctx context.Context, args sdk.AcquireCertificateArgs, originator string) (*sdk.Certificate, error) {
	logging.Child(w.logger, "AcquireCertificate").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO implement me
	panic("implement me")
}

// ListCertificates lists identity certificates belonging to the user, filtered by certifier(s) and type(s).
func (w *Wallet) ListCertificates(ctx context.Context, args sdk.ListCertificatesArgs, originator string) (*sdk.ListCertificatesResult, error) {
	logging.Child(w.logger, "ListCertificates").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO implement me
	panic("implement me")
}

// ProveCertificate proves select fields of an identity certificate, as specified, when requested by a verifier.
func (w *Wallet) ProveCertificate(ctx context.Context, args sdk.ProveCertificateArgs, originator string) (*sdk.ProveCertificateResult, error) {
	logging.Child(w.logger, "ProveCertificate").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO implement me
	panic("implement me")
}

// RelinquishCertificate relinquishes an identity certificate, removing it from the wallet regardless of whether
// the revocation outpoint has become spent.
func (w *Wallet) RelinquishCertificate(ctx context.Context, args sdk.RelinquishCertificateArgs, originator string) (*sdk.RelinquishCertificateResult, error) {
	logging.Child(w.logger, "RelinquishCertificate").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO implement me
	panic("implement me")
}

// DiscoverByIdentityKey discovers identity certificates, issued to a given identity key by a trusted entity.
func (w *Wallet) DiscoverByIdentityKey(ctx context.Context, args sdk.DiscoverByIdentityKeyArgs, originator string) (*sdk.DiscoverCertificatesResult, error) {
	logging.Child(w.logger, "DiscoverByIdentityKey").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO implement me
	panic("implement me")
}

// DiscoverByAttributes discovers identity certificates belonging to other users, where the documents contain
// specific attributes, issued by a trusted entity.
func (w *Wallet) DiscoverByAttributes(ctx context.Context, args sdk.DiscoverByAttributesArgs, originator string) (*sdk.DiscoverCertificatesResult, error) {
	logging.Child(w.logger, "DiscoverByAttributes").DebugContext(ctx, "call", slogx.String("originator", originator))
	// TODO implement me
	panic("implement me")
}

// IsAuthenticated checks the authentication status of the user.
func (w *Wallet) IsAuthenticated(ctx context.Context, _ any, originator string) (*sdk.AuthenticatedResult, error) {
	logging.Child(w.logger, "IsAuthenticated").DebugContext(ctx, "call", slogx.String("originator", originator))
	err := validate.Originator(originator)
	if err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}
	return &sdk.AuthenticatedResult{
		Authenticated: true,
	}, nil
}

// WaitForAuthentication continuously waits until the user is authenticated, returning the result once confirmed.
func (w *Wallet) WaitForAuthentication(ctx context.Context, _ any, originator string) (*sdk.AuthenticatedResult, error) {
	logging.Child(w.logger, "WaitForAuthentication").DebugContext(ctx, "call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	return &sdk.AuthenticatedResult{
		Authenticated: true,
	}, nil
}

// GetHeight retrieves the current height of the blockchain.
func (w *Wallet) GetHeight(ctx context.Context, _ any, originator string) (*sdk.GetHeightResult, error) {
	logging.Child(w.logger, "GetHeight").DebugContext(ctx, "call", slogx.String("originator", originator))
	if w.services == nil {
		return nil, fmt.Errorf("services are not configured for this wallet")
	}

	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	currentHeight, err := w.services.CurrentHeight(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current height: %w", err)
	}

	return &sdk.GetHeightResult{
		Height: currentHeight,
	}, nil
}

// GetHeaderForHeight retrieves the block header of a block at a specified height.
func (w *Wallet) GetHeaderForHeight(ctx context.Context, args sdk.GetHeaderArgs, originator string) (*sdk.GetHeaderResult, error) {
	logging.Child(w.logger, "GetHeaderForHeight").DebugContext(ctx, "call", slogx.String("originator", originator), logging.Number("height", args.Height))
	if w.services == nil {
		return nil, fmt.Errorf("wallet services not configured: cannot retrieve block header")
	}

	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	wdkResult, err := w.services.ChainHeaderByHeight(ctx, args.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to get header for height %d: %w", args.Height, err)
	}

	result, err := mapping.MapGetHeaderResults(wdkResult)
	if err != nil {
		return nil, fmt.Errorf("failed to map get header results: %w", err)
	}

	return result, nil
}

// GetNetwork retrieves the Bitcoin network the client is using (mainnet or testnet).
func (w *Wallet) GetNetwork(ctx context.Context, _ any, originator string) (*sdk.GetNetworkResult, error) {
	logging.Child(w.logger, "GetNetwork").DebugContext(ctx, "call", slogx.String("originator", originator))
	err := validate.Originator(originator)
	if err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	return &sdk.GetNetworkResult{
		Network: sdk.Network(w.chain),
	}, nil
}

// GetVersion retrieves the current version string of the wallet.
func (w *Wallet) GetVersion(ctx context.Context, _ any, originator string) (*sdk.GetVersionResult, error) {
	logging.Child(w.logger, "GetVersion").DebugContext(ctx, "call", slogx.String("originator", originator))
	if err := validate.Originator(originator); err != nil {
		return nil, fmt.Errorf("invalid originator: %w", err)
	}

	return &sdk.GetVersionResult{
		Version: defs.Version,
	}, nil

}

// Close closes the wallet and all the components underneath.
func (w *Wallet) Close() {
	logging.Child(w.logger, "Close").DebugContext(context.Background(), "call")
	w.cleanup()
}

// Destroy is an alias for Close, that is an equivalent for the typescript wallet.destroy() method.
func (w *Wallet) Destroy() {
	logging.Child(w.logger, "Destroy").DebugContext(context.Background(), "call")
	w.Close()
}
